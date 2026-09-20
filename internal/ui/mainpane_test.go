package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

func newTestPane() MainPane {
	m := NewMainPane()
	m.SetSize(80, 24)
	return m
}

func TestMainPane_SyncActiveDevice_NilActive(t *testing.T) {
	m := newTestPane()
	devs := []device.Device{{ID: "abc", Status: device.StatusRunning}}
	m.SyncActiveDevice(devs)
}

func TestMainPane_SyncActiveDevice_Found(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "abc", Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	updated := device.Device{ID: "abc", Status: device.StatusOff}
	m.SyncActiveDevice([]device.Device{updated})
	if m.active == nil || m.active.Status != device.StatusOff {
		t.Error("expected active device status updated")
	}
}

func TestMainPane_SyncActiveDevice_NotFound(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "abc", Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.SyncActiveDevice([]device.Device{{ID: "other"}})
	if m.active != nil {
		t.Error("expected active to be nil when device removed")
	}
}

func TestMainPane_SetInfo(t *testing.T) {
	m := newTestPane()
	info := device.DeviceInfo{Fields: []device.InfoField{
		{Key: "Status", Value: "Running"},
		{Key: "OS", Value: "iOS 17"},
	}}
	m.SetInfo(info)
	if len(m.info.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(m.info.Fields))
	}
}

func TestMainPane_SetFocused(t *testing.T) {
	m := newTestPane()
	m.SetFocused(true)
	m.SetFocused(false)
}

func TestMainPane_LoggingBundle(t *testing.T) {
	m := newTestPane()
	m.SetLoggingBundle("com.example.app")
	if m.LoggingBundle() != "com.example.app" {
		t.Errorf("expected com.example.app, got %q", m.LoggingBundle())
	}
	m.SetLoggingBundle("")
	if m.LoggingBundle() != "" {
		t.Errorf("expected cleared, got %q", m.LoggingBundle())
	}
}

func TestMainPane_Update_DeviceInfoKey_EmitsShowMsg(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Name: "iPhone", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "i"})
	if cmd == nil {
		t.Fatal("expected a command from 'i' press")
	}
	if _, ok := cmd().(ShowDeviceInfoMsg); !ok {
		t.Errorf("expected ShowDeviceInfoMsg, got %T", cmd())
	}
}

func TestMainPane_Update_TabApps_Navigation(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Name: "iPhone", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	apps := []device.App{
		{BundleID: "com.a", Name: "A"},
		{BundleID: "com.b", Name: "B"},
	}
	m.SetApps(apps)

	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.appsIdx != 1 {
		t.Errorf("expected appsIdx 1, got %d", m.appsIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.appsIdx != 0 {
		t.Errorf("expected appsIdx 0, got %d", m.appsIdx)
	}
}

func TestMainPane_Update_TabApps_FilterEsc(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.appsFilter = "test"
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.appsFilter != "" {
		t.Errorf("expected filter cleared, got %q", m.appsFilter)
	}
}

func TestMainPane_Update_TabFiles_HomeEnd(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelFiles
	root := &device.FileNode{
		Path:  "/",
		Name:  "/",
		IsDir: true,
		Children: []device.FileNode{
			{Path: "/a", Name: "a", IsDir: true},
			{Path: "/b", Name: "b", IsDir: true},
		},
	}
	m.SetTree(root)
	m, _ = m.Update(tea.KeyPressMsg{Code: 'G'})
	rows := m.flattenTree()
	if m.treeIdx != len(rows)-1 {
		t.Errorf("expected last row, got %d", m.treeIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g'})
	if m.treeIdx != 0 {
		t.Errorf("expected idx 0, got %d", m.treeIdx)
	}
}

func TestMainPane_View_WithDevice_BothPanels(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{
		ID: "1", Name: "iPhone", Platform: device.PlatformIOS, Status: device.StatusRunning,
	}
	m.SetDevice(dev, nil)
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{{Key: "Status", Value: "Running"}}})
	m.SetApps([]device.App{{BundleID: "com.test", Name: "Test"}})
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetTree(root)

	for _, p := range []mainPanel{panelFiles, panelApps} {
		m.panel = p
		got := m.View()
		if got == "" {
			t.Errorf("panel %d: expected non-empty view", p)
		}
	}
}

func TestMainPane_SyncActiveDevice_UpdatesStatusInfoField(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "d1", Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{
		{Key: "Status", Value: "Running"},
		{Key: "OS", Value: "iOS 17"},
	}})

	m.SyncActiveDevice([]device.Device{{ID: "d1", Status: device.StatusOff}})

	// Status field in info must reflect new status.
	found := false
	for _, f := range m.info.Fields {
		if f.Key == "Status" {
			found = true
			if f.Value != string(device.StatusOff) {
				t.Errorf("Status field: got %q, want %q", f.Value, string(device.StatusOff))
			}
		}
	}
	if !found {
		t.Error("Status field not found in info")
	}
}

func TestMainPane_SetDevice_ClearsLoggingBundle(t *testing.T) {
	m := newTestPane()
	m.SetLoggingBundle("com.old.app")
	dev := &device.Device{ID: "d1", Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	if m.LoggingBundle() != "" {
		t.Errorf("expected loggingBundle cleared by SetDevice, got %q", m.LoggingBundle())
	}
}

func TestMainPane_HasDevice_FalseAfterSetDeviceNil(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "d1", Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	if !m.HasDevice() {
		t.Error("expected HasDevice true")
	}
	m.SetDevice(nil, nil)
	if m.HasDevice() {
		t.Error("expected HasDevice false after SetDevice(nil)")
	}
}

func TestMainPane(t *testing.T) {
	m := NewMainPane()
	m.SetSize(80, 24)

	// 1. Initial State
	if m.HasDevice() {
		t.Error("expected no device initially")
	}
	if !strings.Contains(m.View(), "load") {
		t.Error("expected placeholder view initially")
	}

	// 2. SetDevice
	dev := &device.Device{
		ID:       "test-udid",
		Name:     "iPhone 15",
		Platform: device.PlatformIOS,
		Status:   device.StatusRunning,
		Version:  "17.0",
	}
	m.SetDevice(dev, nil)

	if !m.HasDevice() {
		t.Error("expected HasDevice() to be true")
	}
	if m.panel != panelApps {
		t.Errorf("expected default panel panelApps, got %v", m.panel)
	}

	// 3. Panel toggling
	m, _ = m.Update(tea.KeyPressMsg{Text: "3"})
	if m.panel != panelFiles {
		t.Errorf("expected panelFiles, got %v", m.panel)
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	if m.panel != panelApps {
		t.Errorf("expected panelApps, got %v", m.panel)
	}

	// 4. App Filtering
	m.panel = panelApps
	apps := []device.App{
		{BundleID: "com.apple.Maps", ShortVersion: "3.0"},
		{BundleID: "com.google.Maps", ShortVersion: "6.0"},
		{BundleID: "com.spotify.music", ShortVersion: "8.0"},
	}
	m.SetApps(apps)

	m, _ = m.Update(tea.KeyPressMsg{Text: "/"}) // Enter filtering
	if !m.appsFiltering {
		t.Error("expected appsFiltering to be true")
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "m"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "a"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "p"})
	if m.appsFilter != "map" {
		t.Errorf("expected filter 'map', got %q", m.appsFilter)
	}

	filtered := m.filteredApps()
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered apps, got %d", len(filtered))
	}

	// 5. File Tree
	m.panel = panelFiles
	root := &device.FileNode{
		Path:  "/",
		Name:  "/",
		IsDir: true,
		Children: []device.FileNode{
			{Path: "/data", Name: "data", IsDir: true},
			{Path: "/System", Name: "System", IsDir: true},
		},
	}
	m.SetTree(root)

	// Move down and expand
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // Select "data"
	if m.treeIdx != 1 {
		t.Errorf("expected treeIdx 1, got %d", m.treeIdx)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // Toggle "data"
	if !m.expanded["/data"] {
		t.Error("expected /data to be expanded")
	}

	// 6. App logging toggle
	m.panel = panelApps
	m.SetApps([]device.App{{BundleID: "com.apple.Maps", Name: "Maps"}})
	_, cmd := m.Update(tea.KeyPressMsg{Text: "l"})
	if cmd == nil {
		t.Fatal("expected a command from 'l' press")
	}
	if _, ok := cmd().(StartAppLoggingMsg); !ok {
		t.Errorf("expected StartAppLoggingMsg, got %T", cmd())
	}
	m.SetLoggingBundle("com.apple.Maps")
	_, cmd = m.Update(tea.KeyPressMsg{Text: "l"})
	if _, ok := cmd().(StopAppLoggingMsg); !ok {
		t.Errorf("expected StopAppLoggingMsg, got %T", cmd())
	}
}
