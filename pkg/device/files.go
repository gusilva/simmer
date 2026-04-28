package device

import (
	"context"
	"time"
)

// FileNode is a node in a device's filesystem tree.
type FileNode struct {
	Name        string
	Path        string
	IsDir       bool
	Size        int64 // bytes; 0 for dirs unless known
	Modified    time.Time
	Permissions string     // e.g. "-rw-r--r--"
	Children    []FileNode // nil for files; possibly nil for not-yet-loaded dirs
}

// FileSystem provides access to a device's filesystem. Implementations may
// hit real device tooling (e.g. `xcrun simctl get_app_container`) or return
// fixture data for development.
type FileSystem interface {
	// Tree returns the root FileNode for the given device. The returned node
	// itself is a directory whose Children describe the top-level entries.
	Tree(ctx context.Context, dev Device) (FileNode, error)
}

// MockFileSystem returns a hardcoded fixture tree useful for UI development
// before real device-filesystem access is wired up.
type MockFileSystem struct{}

// NewMockFileSystem returns a FileSystem backed by static fixture data.
func NewMockFileSystem() FileSystem { return &MockFileSystem{} }

// Tree returns a fixed iOS-shaped tree rooted at /private/var/mobile.
func (m *MockFileSystem) Tree(_ context.Context, _ Device) (FileNode, error) {
	mod := time.Date(2026, 4, 27, 14, 18, 2, 0, time.UTC)

	file := func(name string, size int64) FileNode {
		return FileNode{
			Name:        name,
			Path:        name,
			IsDir:       false,
			Size:        size,
			Modified:    mod,
			Permissions: "-rw-r--r--",
		}
	}
	dir := func(name string, children ...FileNode) FileNode {
		return FileNode{
			Name:        name,
			Path:        name,
			IsDir:       true,
			Modified:    mod,
			Permissions: "drwxr-xr-x",
			Children:    children,
		}
	}

	return dir("/private/var/mobile",
		dir("Applications",
			dir("com.acme.fieldkit",
				file("FieldKit.app", 24_800_000),
				dir("Documents",
					file("cache.sqlite", 1_200_000),
					file("sync.log", 482_000),
					file("workorders.json", 88_000),
				),
				dir("Library"),
				dir("tmp"),
			),
			dir("com.acme.dispatch"),
			dir("com.example.notes"),
		),
		dir("Library"),
		dir("Media"),
		dir("tmp"),
		dir("var"),
	), nil
}
