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

// Tree walks the simulator's data directory and returns the root node.
func (f *IOSRootFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	if dev.Platform != PlatformIOS {
		return FileNode{}, fmt.Errorf("ios filesystem: unsupported platform %s", dev.Platform)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return FileNode{}, fmt.Errorf("user home: %w", err)
	}
	root := filepath.Join(home,
		"Library", "Developer", "CoreSimulator",
		"Devices", dev.ID, "data",
	)
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

// Tree resolves the app's data container path and walks it up to MaxDepth.
func (f *IOSAppFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	out, err := exec.CommandContext(ctx,
		"xcrun", "simctl", "get_app_container", dev.ID, f.BundleID, "data",
	).Output()
	if err != nil {
		return FileNode{}, fmt.Errorf("get_app_container %s %s: %w", dev.ID, f.BundleID, err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return FileNode{}, fmt.Errorf("get_app_container returned empty path for %s", f.BundleID)
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

// FileSystemFor returns the simulator FileSystem serving scope: the app's
// local container for ScopeAppSandbox, the simulator's data root otherwise.
func (m *iosManager) FileSystemFor(scope FileScope, target string) FileSystem {
	switch {
	case scope == ScopeAppSandbox && target != "":
		return NewIOSAppFileSystem(target)
	case scope == ScopePath && target != "":
		return NewIOSPathFileSystem(target)
	case scope == ScopeDirectory && target != "":
		return &IOSPathFileSystem{Path: target, MaxDepth: 1}
	default:
		return NewIOSRootFileSystem()
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
