package device

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// walkNode walks the filesystem at path, returning a FileNode tree pruned at
// maxDepth. Permission errors on a directory are swallowed (the dir is
// returned with no children) so an unreadable subtree doesn't fail the whole
// walk. Context cancellation aborts and surfaces the ctx error.
func walkNode(ctx context.Context, path string, info os.FileInfo, depth, maxDepth int) (FileNode, error) {
	if err := ctx.Err(); err != nil {
		return FileNode{}, err
	}

	n := FileNode{
		Name:        info.Name(),
		Path:        path,
		IsDir:       info.IsDir(),
		Size:        info.Size(),
		Modified:    info.ModTime(),
		Permissions: info.Mode().String(),
	}
	if !info.IsDir() || depth >= maxDepth {
		return n, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		// Unreadable dir: keep as-is with no children.
		return n, nil
	}

	children := make([]FileNode, 0, len(entries))
	for _, e := range entries {
		ei, err := e.Info()
		if err != nil {
			continue
		}
		child, err := walkNode(ctx, filepath.Join(path, e.Name()), ei, depth+1, maxDepth)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return FileNode{}, err
			}
			continue
		}
		children = append(children, child)
	}

	// Directories first, then files; alphabetical within each group.
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].IsDir != children[j].IsDir {
			return children[i].IsDir
		}
		return children[i].Name < children[j].Name
	})

	n.Children = children
	return n, nil
}

// IOSRootFileSystem walks the simulator's full data directory (the simulated
// filesystem root at ~/Library/Developer/CoreSimulator/Devices/<UDID>/data/).
type IOSRootFileSystem struct {
	MaxDepth int
}

// NewIOSRootFileSystem returns a FileSystem rooted at the simulator's data dir.
func NewIOSRootFileSystem() FileSystem {
	return &IOSRootFileSystem{MaxDepth: 2}
}

// simulatorDataRoot returns a simulator's data directory on this Mac, or ""
// when the home directory cannot be determined.
func simulatorDataRoot(dev Device) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices", dev.ID, "data")
}

// Tree walks the simulator's data directory and returns the root node.
func (f *IOSRootFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	if dev.Platform != PlatformIOS {
		return FileNode{}, fmt.Errorf("ios filesystem: unsupported platform %s", dev.Platform)
	}
	root := simulatorDataRoot(dev)
	if root == "" {
		return FileNode{}, fmt.Errorf("user home unavailable")
	}

	info, err := os.Stat(root)
	if err != nil {
		return FileNode{}, fmt.Errorf("stat %s: %w", root, err)
	}
	maxD := f.MaxDepth
	if maxD <= 0 {
		maxD = 2
	}
	node, err := walkNode(ctx, root, info, 0, maxD)
	if err != nil {
		return FileNode{}, err
	}
	node.Name = "/"
	node.Path = root
	return node, nil
}

// IOSAppFileSystem resolves a single app's data container via
// `xcrun simctl get_app_container` and walks it locally, scoped to one
// bundle.
type IOSAppFileSystem struct {
	BundleID string
	MaxDepth int
}

// NewIOSAppFileSystem returns a FileSystem scoped to the given app's data
// container on an iOS simulator.
func NewIOSAppFileSystem(bundleID string) FileSystem {
	return &IOSAppFileSystem{BundleID: bundleID, MaxDepth: 4}
}

// iosAppContainer resolves an app's data container on a simulator. The
// container is a directory on this Mac, but only simctl knows which one.
func iosAppContainer(ctx context.Context, dev Device, bundleID string) (string, error) {
	out, err := exec.CommandContext(ctx,
		"xcrun", "simctl", "get_app_container", dev.ID, bundleID, "data",
	).Output()
	if err != nil {
		return "", fmt.Errorf("get_app_container %s %s: %w", dev.ID, bundleID, err)
	}

	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("get_app_container returned empty path for %s", bundleID)
	}

	return root, nil
}

// Tree resolves the app's data container path and walks it up to MaxDepth.
func (f *IOSAppFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	root, err := iosAppContainer(ctx, dev, f.BundleID)
	if err != nil {
		return FileNode{}, err
	}

	info, err := os.Stat(root)
	if err != nil {
		return FileNode{}, fmt.Errorf("stat %s: %w", root, err)
	}

	maxD := f.MaxDepth
	if maxD <= 0 {
		maxD = 4
	}
	node, err := walkNode(ctx, root, info, 0, maxD)
	if err != nil {
		return FileNode{}, err
	}
	node.Name = f.BundleID
	return node, nil
}

// FileSystemFor returns the simulator FileSystem serving a target.
func (m *iosManager) FileSystemFor(t FileTarget, d Depth) FileSystem {
	depth := 6
	if d == OneLevel {
		depth = 1
	}

	switch {
	case t.BundleID() != "":
		return NewIOSAppFileSystem(t.BundleID())
	case t.Path() != "":
		return &IOSPathFileSystem{Path: t.Path(), MaxDepth: depth}
	default:
		return NewIOSRootFileSystem()
	}
}

// StorageRoots reports the directories a target covers. A simulator keeps
// an app's storage in one container, which simctl resolves.
func (m *iosManager) StorageRoots(ctx context.Context, dev Device, t FileTarget) ([]string, error) {
	switch {
	case t.BundleID() != "":
		root, err := iosAppContainer(ctx, dev, t.BundleID())
		if err != nil {
			return nil, err
		}
		return []string{root}, nil
	case t.Path() != "":
		return []string{t.Path()}, nil
	default:
		return []string{simulatorDataRoot(dev)}, nil
	}
}

// IOSPathFileSystem walks a named directory of a simulator. A simulator's
// storage is a directory on this Mac, so the path is a host path.
type IOSPathFileSystem struct {
	Path     string
	MaxDepth int
}

// NewIOSPathFileSystem returns a FileSystem rooted at a host path.
func NewIOSPathFileSystem(path string) FileSystem {
	return &IOSPathFileSystem{Path: path, MaxDepth: 6}
}

func (f *IOSPathFileSystem) Tree(ctx context.Context, _ Device) (FileNode, error) {
	path := expandPath(f.Path)
	info, err := os.Stat(path)
	if err != nil {
		return FileNode{}, fmt.Errorf("stat %s: %w", path, err)
	}
	return walkNode(ctx, path, info, 0, f.MaxDepth)
}
