package ui

import (
	"testing"

	"simmer/internal/device"
)

func TestMainPane_HandleClick_NoDevice_NoPanic(t *testing.T) {
	m := newTestPane()
	m.HandleClick(5, 5) // active == nil: must be a no-op, not a panic
	if m.panel != panelApps {
		t.Errorf("expected default panel unchanged, got %v", m.panel)
	}
}

func TestMainPane_HandleClick_AppsLabel_SwitchesToApps(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelFiles

	m.HandleClick(5, 4) // ly=3: "[2] Apps" label row
	if m.panel != panelApps {
		t.Errorf("expected panel switched to Apps, got %v", m.panel)
	}
}

func TestMainPane_HandleClick_FilesLabel_SwitchesToFiles(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil) // defaults to panelApps

	// innerH=22, content=max(innerH-6,1)=16; Files label row is ly=5+content=21 -> y=22.
	m.HandleClick(5, 22)
	if m.panel != panelFiles {
		t.Errorf("expected panel switched to Files, got %v", m.panel)
	}
}

func TestMainPane_HandleClick_AppsRow_SelectsClickedApp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil) // panelApps by default
	m.SetApps([]device.App{
		{BundleID: "com.a", Name: "A"},
		{BundleID: "com.b", Name: "B"},
	})

	// content=16, listH=14, offset=0; block=2 -> idx=1 (second app row).
	// block = ly-4, ly = y-1, so y = 4+block+1 = 7 for block=2.
	m.HandleClick(5, 7)
	if m.panel != panelApps {
		t.Errorf("expected panel still Apps, got %v", m.panel)
	}
	if m.appsIdx != 1 {
		t.Errorf("expected appsIdx 1, got %d", m.appsIdx)
	}
}

func TestMainPane_HandleClick_FilesRow_SelectsClickedNode(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	root := &device.FileNode{
		Path: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/a.txt", Name: "a.txt"},
			{Path: "/b.txt", Name: "b.txt"},
		},
	}
	m.SetDevice(dev, root)
	m.panel = panelFiles

	// content=16, visibleH=15, offset=0; block=2 -> idx=1 (second tree row).
	// block = ly-6, ly = y-1, so y = 6+block+1 = 9 for block=2.
	m.HandleClick(5, 9)
	if m.panel != panelFiles {
		t.Errorf("expected panel still Files, got %v", m.panel)
	}
	if m.treeIdx != 1 {
		t.Errorf("expected treeIdx 1, got %d", m.treeIdx)
	}
}

func TestMainPane_HandleClick_FilesRow_PreviewColumn_DoesNotChangeSelection(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	root := &device.FileNode{
		Path: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/a.txt", Name: "a.txt"},
			{Path: "/b.txt", Name: "b.txt"},
		},
	}
	m.SetDevice(dev, root)
	m.panel = panelFiles

	// treeW for innerW=78 is 40; x=60 lands in the preview column.
	m.HandleClick(60, 9)
	if m.panel != panelFiles {
		t.Errorf("expected panel still Files (click still focuses the region), got %v", m.panel)
	}
	if m.treeIdx != 0 {
		t.Errorf("expected treeIdx unchanged (0), got %d", m.treeIdx)
	}
}

func TestMainPane_HandleClick_OutOfBounds_NoOp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.appsIdx = 0

	m.HandleClick(0, 0) // border corner
	if m.appsIdx != 0 || m.panel != panelApps {
		t.Errorf("expected no state change from border click, got panel=%v idx=%d", m.panel, m.appsIdx)
	}
}
