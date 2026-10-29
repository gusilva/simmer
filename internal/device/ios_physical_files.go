package device

import (
	"context"
	"fmt"
	"os"
	gopath "path"
	"sort"
	"strings"

	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/house_arrest"

	"simmer/internal/logging"
)

// IOSPhysicalFileSystem browses the media partition of a physical iOS device
// via the AFC service (com.apple.afc). Accessible without jailbreak.
// Exposed paths include /DCIM, /iTunes_Control, /PhotoData, etc.
type IOSPhysicalFileSystem struct {
	MaxDepth int
	logger   *logging.Logger
}

// NewIOSPhysicalFileSystem returns a FileSystem for the AFC media partition.
func NewIOSPhysicalFileSystem(logger *logging.Logger) FileSystem {
	return &IOSPhysicalFileSystem{MaxDepth: 3, logger: logger}
}

func (f *IOSPhysicalFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		return FileNode{}, err
	}
	client, err := afc.New(entry)
	f.logger.LogExec("go-ios", []string{"afc", "connect"}, "", err)
	if err != nil {
		return FileNode{}, fmt.Errorf("afc: %w", err)
	}
	defer client.Close()

	maxDepth := f.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	root, err := buildAFCTree(ctx, client, "/", 0, maxDepth)
	root.Name = "/"
	return root, err
}

// IOSPhysicalAppFileSystem browses an app's container on a physical iOS device
// via the house_arrest service. Exposes the app's Documents, Library and tmp.
type IOSPhysicalAppFileSystem struct {
	BundleID string
	MaxDepth int
	logger   *logging.Logger
}

// NewIOSPhysicalAppFileSystem returns a FileSystem for a single app's container.
func NewIOSPhysicalAppFileSystem(bundleID string, logger *logging.Logger) FileSystem {
	return &IOSPhysicalAppFileSystem{BundleID: bundleID, MaxDepth: 4, logger: logger}
}

func (f *IOSPhysicalAppFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		return FileNode{}, err
	}
	client, err := house_arrest.New(entry, f.BundleID)
	f.logger.LogExec("go-ios", []string{"house_arrest", f.BundleID}, "", err)
	if err != nil {
		return FileNode{}, fmt.Errorf("house_arrest %s: %w", f.BundleID, err)
	}
	defer client.Close()

	maxDepth := f.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 4
	}
	root, err := buildAFCTree(ctx, client, "/", 0, maxDepth)
	root.Name = f.BundleID
	return root, err
}

// buildAFCTree recursively lists an AFC path up to maxDepth levels.
func buildAFCTree(ctx context.Context, client *afc.Client, path string, depth, maxDepth int) (FileNode, error) {
	names, err := client.List(path)
	if err != nil {
		return FileNode{Name: gopath.Base(path), Path: path, IsDir: true},
			fmt.Errorf("list %s: %w", path, err)
	}

	node := FileNode{
		Name:  gopath.Base(path),
		Path:  path,
		IsDir: true,
	}

	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		childPath := strings.TrimSuffix(path, "/") + "/" + name

		info, err := client.Stat(childPath)
		if err != nil {
			continue
		}

		child := FileNode{
			Name:        name,
			Path:        childPath,
			IsDir:       info.IsDir(),
			Size:        info.Size,
			Permissions: os.FileMode(info.Mode).String(),
		}
		if info.IsDir() && depth+1 < maxDepth {
			sub, _ := buildAFCTree(ctx, client, childPath, depth+1, maxDepth)
			child.Children = sub.Children
		}
		node.Children = append(node.Children, child)
	}

	sort.SliceStable(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return a.Name < b.Name
	})

	return node, nil
}

// FileSystemFor returns the physical-iOS FileSystem serving a target.
func (m *physicalIOSManager) FileSystemFor(t FileTarget, d Depth) FileSystem {
	depth := 6
	if d == OneLevel {
		depth = 1
	}

	switch {
	case t.BundleID() != "":
		return NewIOSPhysicalAppFileSystem(t.BundleID(), m.logger)
	case t.Path() != "":
		return &IOSPhysicalPathFileSystem{Path: t.Path(), MaxDepth: depth, logger: m.logger}
	default:
		return NewIOSPhysicalFileSystem(m.logger)
	}
}

// StorageRoots reports the directories a target covers. house_arrest vends
// an app's container as its own filesystem, so the root inside it is "/".
func (m *physicalIOSManager) StorageRoots(_ context.Context, _ Device, t FileTarget) ([]string, error) {
	switch {
	case t.BundleID() != "":
		return []string{"/"}, nil
	case t.Path() != "":
		return []string{t.Path()}, nil
	default:
		return []string{"/"}, nil
	}
}

// IOSPhysicalPathFileSystem walks a named directory of the media partition
// over AFC. Only what AFC vends is reachable: there is no shell on a
// non-jailbroken device, so a path outside the media partition cannot be
// listed whatever its permissions.
type IOSPhysicalPathFileSystem struct {
	Path     string
	MaxDepth int
	logger   *logging.Logger
}

func (f *IOSPhysicalPathFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		return FileNode{}, err
	}
	client, err := afc.New(entry)
	f.logger.LogExec("go-ios", []string{"afc", "connect"}, "", err)
	if err != nil {
		return FileNode{}, fmt.Errorf("afc: %w", err)
	}
	defer func() { _ = client.Close() }()

	maxDepth := f.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 6
	}
	root, err := buildAFCTree(ctx, client, f.Path, 0, maxDepth)
	if err != nil {
		return FileNode{}, fmt.Errorf("%s: %w", f.Path, err)
	}
	return root, nil
}
