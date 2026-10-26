package device

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/electricbubble/gadb"

	"simmer/internal/logging"
)

// AndroidPhysicalFileSystem lists external storage on a physical Android device
// via gadb's List (SYNC protocol). Accessible without root.
type AndroidPhysicalFileSystem struct {
	RootPath string
	MaxDepth int
	logger   *logging.Logger
}

// [TODO] fix this path resolution logic to be more robust and support more devices.
// For example, some devices may have multiple external storage locations, or the path may differ based on Android version or manufacturer.
// We could potentially use the "getExternalFilesDirs" ADB command to discover available external storage paths dynamically.
// NewAndroidPhysicalFileSystem returns a FileSystem rooted at /storage/emulated/0.
func NewAndroidPhysicalFileSystem(logger *logging.Logger) FileSystem {
	return &AndroidPhysicalFileSystem{RootPath: "/storage/emulated/0", MaxDepth: 4, logger: logger}
}

func (f *AndroidPhysicalFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
	d, err := gadbDevice(dev.ID)
	if err != nil {
		return FileNode{}, err
	}

	rootPath := f.RootPath
	if rootPath == "" {
		rootPath = "/storage/emulated/0"
	}
	maxDepth := f.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 4
	}
	return buildGadbTree(ctx, f.logger, d, rootPath, 0, maxDepth)
}

func buildGadbTree(ctx context.Context, logger *logging.Logger, d gadb.Device, path string, depth, maxDepth int) (FileNode, error) {
	entries, err := d.List(path)
	logger.LogExec("adb sync", []string{"ls", path}, "", err)
	if err != nil {
		return FileNode{Name: filepath.Base(path), Path: path, IsDir: true},
			fmt.Errorf("list %s: %w", path, err)
	}

	name := filepath.Base(path)
	if path == "/" {
		name = "/"
	}
	node := FileNode{
		Name:  name,
		Path:  path,
		IsDir: true,
	}

	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		if e.Name == "." || e.Name == ".." {
			continue
		}
		childPath := path + "/" + e.Name
		child := FileNode{
			Name:        e.Name,
			Path:        childPath,
			IsDir:       e.IsDir(),
			Size:        int64(e.Size),
			Modified:    e.LastModified,
			Permissions: e.Mode.String(),
		}
		if e.IsDir() && depth+1 < maxDepth {
			sub, _ := buildGadbTree(ctx, logger, d, childPath, depth+1, maxDepth)
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

// FileSystemFor returns the physical-Android FileSystem serving scope.
//
// An app sandbox is read exactly as on an emulator: run-as needs the package
// to be debuggable, not the device to be rooted, and the sandbox FileSystem
// already returns whichever of the two storage areas it can read.
//
// There is deliberately no fallback to external storage here. It used to
// fire when both areas were refused and answer "this app's sandbox" with the
// whole of /storage/emulated/0 — a wrong tree that surfaced as an empty
// result, which reads as "this app has no databases".
func (m *physicalAndroidManager) FileSystemFor(scope FileScope, target string) FileSystem {
	switch {
	case scope == ScopeAppSandbox && target != "":
		return NewAndroidFileSystem(target, m.logger)
	case scope == ScopePath && target != "":
		return NewAndroidPathFileSystem(target, m.logger)
	case scope == ScopeDirectory && target != "":
		return NewAndroidDirFileSystem(target, m.logger)
	default:
		// Shared storage is what a physical device's root listing can
		// reach without root.
		return NewAndroidPhysicalFileSystem(m.logger)
	}
}
