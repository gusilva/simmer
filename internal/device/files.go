package device

import (
	"context"
	"strings"
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

// FileScope selects which part of a device's filesystem a tree walk covers.
type FileScope int

const (
	// ScopeDeviceRoot walks the device's own filesystem from its root.
	ScopeDeviceRoot FileScope = iota
	// ScopeAppSandbox walks one app's own storage, identified by its bundle
	// id / package name. Managers that cannot reach a private container
	// (physical Android with a release-signed app) fall back to the storage
	// they can read.
	ScopeAppSandbox
	// ScopePath walks an arbitrary directory on the device, identified by
	// its absolute path. Shared storage lives outside any app's sandbox, so
	// a path is the only way to name it.
	ScopePath
	// ScopeDirectory lists one directory's entries without descending into
	// them. This is what an interactive tree expands with: a walk would cost
	// an unbounded listing to show one level.
	ScopeDirectory
)

// FileSystemProvider is an optional interface a Manager may implement to
// supply the FileSystem that serves a given scope. The manager picks the
// adapter because only it knows which transport its devices support
// (simctl paths, AFC, adb run-as, …).
//
// target names what the scope leaves open: a bundle id for ScopeAppSandbox,
// an absolute device path for ScopePath, and nothing for ScopeDeviceRoot.
type FileSystemProvider interface {
	Platform() Platform
	FileSystemFor(scope FileScope, target string) FileSystem
}

// FileTree walks dev's filesystem at the given scope and returns the root
// node. target is a bundle id for ScopeAppSandbox, an absolute path for
// ScopePath, and ignored for ScopeDeviceRoot. Routing prefers a
// KindedManager whose Kind matches dev.Kind.
func (c *Coordinator) FileTree(ctx context.Context, dev Device, scope FileScope, target string) (FileNode, error) {
	p, err := pickCapability[FileSystemProvider](c.Managers, dev.Platform, dev.Kind, "file system provider")
	if err != nil {
		return FileNode{}, err
	}
	return p.FileSystemFor(scope, target).Tree(ctx, dev)
}

// IsDatabaseFile reports whether name looks like a SQLite database by
// extension. Used to decide which files in a device file tree can be opened
// in the SQLite viewer or queried headlessly.
func IsDatabaseFile(name string) bool {
	lname := strings.ToLower(name)
	return strings.HasSuffix(lname, ".db") ||
		strings.HasSuffix(lname, ".sqlite") ||
		strings.HasSuffix(lname, ".sqlite3")
}
