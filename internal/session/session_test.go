package session

import (
	"context"
	"testing"

	"simmer/internal/config"
	"simmer/internal/device"
)

func testConfig() config.Config { return config.Default() }

// fakeManager is a device.Manager that serves canned data, so Session can be
// exercised without Xcode, the Android SDK or a connected phone. It stands at
// the same seam as the real managers.
type fakeManager struct {
	platform device.Platform
	kind     device.DeviceKind
	devices  []device.Device
	apps     []device.App
	versions map[string]string
	rn       map[string]bool
	tree     device.FileNode
	appTree  device.FileNode
	pathTree device.FileNode

	listAppsErr error
}

func (f *fakeManager) ListDevices(context.Context) ([]device.Device, error) {
	return f.devices, nil
}
func (f *fakeManager) Platform() device.Platform { return f.platform }
func (f *fakeManager) Kind() device.DeviceKind   { return f.kind }
func (f *fakeManager) ListApps(context.Context, string) ([]device.App, error) {
	return f.apps, f.listAppsErr
}
func (f *fakeManager) FetchAppVersions(_ context.Context, _ string, bundleIDs []string) map[string]string {
	out := make(map[string]string)
	for _, id := range bundleIDs {
		if v, ok := f.versions[id]; ok {
			out[id] = v
		}
	}
	return out
}
func (f *fakeManager) DetectReactNative(_ context.Context, _ string, app device.App) bool {
	return f.rn[app.BundleID]
}
func (f *fakeManager) FileSystemFor(scope device.FileScope, _ string) device.FileSystem {
	switch scope {
	case device.ScopeAppSandbox:
		return fakeFS{f.appTree}
	case device.ScopePath:
		return fakeFS{f.pathTree}
	default:
		return fakeFS{f.tree}
	}
}

type fakeFS struct{ root device.FileNode }

func (f fakeFS) Tree(context.Context, device.Device) (device.FileNode, error) {
	return f.root, nil
}

func TestListDevicesRecordsRoster(t *testing.T) {
	devs := []device.Device{dev("X", "Pixel_8", device.PlatformAndroid, device.StatusRunning)}
	s := New(device.NewCoordinator(&fakeManager{
		platform: device.PlatformAndroid,
		kind:     device.KindVirtual,
		devices:  devs,
	}), nil, testConfig())

	res := s.ListDevices(context.Background())
	if len(res.Devices) != 1 {
		t.Fatalf("discovered %d devices, want 1", len(res.Devices))
	}

	got, err := s.Resolve("Pixel")
	if err != nil {
		t.Fatalf("resolve after discovery: %v", err)
	}
	if got.ID != "X" {
		t.Errorf("resolved %q, want X", got.ID)
	}
}

func TestFileTreeRoutesByScope(t *testing.T) {
	root := device.FileNode{Name: "/", IsDir: true}
	sandbox := device.FileNode{Name: "sandbox", IsDir: true}
	s := New(device.NewCoordinator(&fakeManager{
		platform: device.PlatformAndroid,
		kind:     device.KindVirtual,
		tree:     root,
		appTree:  sandbox,
	}), nil, testConfig())

	target := dev("X", "Pixel_8", device.PlatformAndroid, device.StatusRunning)

	got, err := s.FileTree(context.Background(), target, device.ScopeDeviceRoot, "")
	if err != nil || got.Name != "/" {
		t.Fatalf("device root: got %q, err %v", got.Name, err)
	}

	got, err = s.FileTree(context.Background(), target, device.ScopeAppSandbox, "com.acme.app")
	if err != nil || got.Name != "sandbox" {
		t.Fatalf("app sandbox: got %q, err %v", got.Name, err)
	}
}

func TestFileTreeWalksANamedPath(t *testing.T) {
	// Shared storage belongs to no app, so it is reachable only by path.
	download := device.FileNode{Name: "Download", Path: "/storage/emulated/0/Download", IsDir: true}
	s := New(device.NewCoordinator(&fakeManager{
		platform: device.PlatformAndroid,
		kind:     device.KindVirtual,
		pathTree: download,
	}), nil, testConfig())

	got, err := s.FileTree(context.Background(), androidDevice(device.KindVirtual), device.ScopePath, download.Path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Path != download.Path {
		t.Errorf("got %q, want %q", got.Path, download.Path)
	}
}

func TestFileTreeUnroutedPlatformErrors(t *testing.T) {
	s := New(device.NewCoordinator(), nil, testConfig())
	_, err := s.FileTree(context.Background(), dev("X", "iPhone", device.PlatformIOS, device.StatusOff), device.ScopeDeviceRoot, "")
	if err == nil {
		t.Fatal("expected an error when no manager serves the platform")
	}
}
