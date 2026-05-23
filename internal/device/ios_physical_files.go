package device

import (
	"context"
	"fmt"
	"os"
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
		return FileNode{Name: afcBase(path), Path: path, IsDir: true},
			fmt.Errorf("list %s: %w", path, err)
	}

	node := FileNode{
		Name:  afcBase(path),
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

// afcBase returns the last path component, treating "/" as its own name.
func afcBase(path string) string {
	if path == "/" {
		return "/"
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
