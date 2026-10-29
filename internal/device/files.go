package device

import (
	"context"
	"fmt"
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

// FileTarget names what a file operation covers. Construct it with
// DeviceRoot, AppStorage or Directory — the zero value is invalid, so an
// unset target cannot quietly become "the whole device".
//
// An app's storage is a target rather than a path because on Android it is
// two paths reached two different ways, and only the device layer should
// know that.
type FileTarget struct {
	bundleID string
	path     string
	device   bool
}

// DeviceRoot targets the device's own filesystem.
func DeviceRoot() FileTarget { return FileTarget{device: true} }

// AppStorage targets everything one app owns.
func AppStorage(bundleID string) FileTarget { return FileTarget{bundleID: bundleID} }

// Directory targets one absolute directory on the device.
func Directory(path string) FileTarget { return FileTarget{path: path} }

// BundleID is the app whose storage is targeted, empty for other targets.
func (t FileTarget) BundleID() string { return t.bundleID }

// Path is the directory targeted, empty for other targets.
func (t FileTarget) Path() string { return t.path }

// IsDeviceRoot reports whether the target is the device's own filesystem.
func (t FileTarget) IsDeviceRoot() bool { return t.device }

func (t FileTarget) String() string {
	switch {
	case t.device:
		return "device root"
	case t.bundleID != "":
		return t.bundleID + " storage"
	default:
		return t.path
	}
}

// validate rejects targets that name nothing, which previously fell through
// to a device-root listing and answered a question nobody asked.
func (t FileTarget) validate() error {
	switch {
	case t.device:
		return nil
	case t.bundleID != "":
		return nil
	case t.path == "":
		return fmt.Errorf("file target names neither an app nor a directory")
	case !strings.HasPrefix(t.path, "/"):
		return fmt.Errorf("directory must be absolute, got %q", t.path)
	default:
		return nil
	}
}

// Depth says how far below a target to read.
type Depth int

const (
	// FullDepth reads everything under the target, subject to whatever cap
	// the adapter needs to stay responsive.
	FullDepth Depth = iota
	// OneLevel reads a directory's own entries and stops. This is what an
	// interactive tree expands with: a walk would cost an unbounded listing
	// to show one level.
	OneLevel
)

// FileSystemProvider is an optional interface a Manager may implement to
// supply the FileSystem that serves a target. The manager picks the adapter
// because only it knows which transport its devices support (simctl paths,
// AFC, adb run-as, …).
type FileSystemProvider interface {
	Platform() Platform
	FileSystemFor(t FileTarget, d Depth) FileSystem
	// StorageRoots reports the directories a target covers, which is what a
	// search has to iterate. Resolving them can require the device (an iOS
	// simulator's container path comes from simctl).
	StorageRoots(ctx context.Context, dev Device, t FileTarget) ([]string, error)
}

// FileTree reads dev's filesystem at the given target and depth.
func (c *Coordinator) FileTree(ctx context.Context, dev Device, t FileTarget, d Depth) (FileNode, error) {
	if err := t.validate(); err != nil {
		return FileNode{}, err
	}

	p, err := pickCapability[FileSystemProvider](c.Managers, dev.Platform, dev.Kind, "file system provider")
	if err != nil {
		return FileNode{}, err
	}

	return p.FileSystemFor(t, d).Tree(ctx, dev)
}

// StorageRoots reports the directories a target covers on dev.
func (c *Coordinator) StorageRoots(ctx context.Context, dev Device, t FileTarget) ([]string, error) {
	if err := t.validate(); err != nil {
		return nil, err
	}

	p, err := pickCapability[FileSystemProvider](c.Managers, dev.Platform, dev.Kind, "file system provider")
	if err != nil {
		return nil, err
	}

	return p.StorageRoots(ctx, dev, t)
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
