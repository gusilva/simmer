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

func TestMainPane_SetInfo_ClampsIdx(t *testing.T) {
	m := newTestPane()
	m.infoIdx = 10
	info := device.DeviceInfo{Fields: []device.InfoField{{Key: "k", Value: "v"}}}
	m.SetInfo(info)
	if m.infoIdx != 0 {
		t.Errorf("expected infoIdx clamped to 0, got %d", m.infoIdx)
	}
}

func TestMainPane_SetFocused(t *testing.T) {
	m := newTestPane()
	m.SetFocused(true)
	m.SetFocused(false)
}

func TestMainPane_LogBundle(t *testing.T) {
	m := newTestPane()
	m.SetLogBundle("com.example.app")
	if m.LogBundle() != "com.example.app" {
		t.Errorf("expected com.example.app, got %q", m.LogBundle())
	}
}

func TestMainPane_Update_TabInfo_Navigation(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Name: "iPhone", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{{Key: "k", Value: "v"}, {Key: "k2", Value: "v2"}}})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.infoIdx != 1 {
		t.Errorf("expected infoIdx 1, got %d", m.infoIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.infoIdx != 0 {
		t.Errorf("expected infoIdx 0, got %d", m.infoIdx)
	}
}

func TestMainPane_Update_TabApps_Navigation(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Name: "iPhone", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
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
	m.tab = TabApps
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
	m.tab = TabFiles
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

func TestMainPane_View_WithDevice_AllTabs(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{
		ID: "1", Name: "iPhone", Platform: device.PlatformIOS, Status: device.StatusRunning,
	}
	m.SetDevice(dev, nil)
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{{Key: "Status", Value: "Running"}}})
	m.SetApps([]device.App{{BundleID: "com.test", Name: "Test"}})
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetTree(root)

	for _, tab := range []MainTab{TabInfo, TabApps, TabFiles, TabLogs} {
		m.tab = tab
		got := m.View()
		if got == "" {
			t.Errorf("tab %d: expected non-empty view", tab)
		}
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
	if m.tab != TabInfo {
		t.Errorf("expected default tab TabInfo, got %v", m.tab)
	}

	// 3. Tab Switching
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	if m.tab != TabApps {
		t.Errorf("expected TabApps, got %v", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "3"})
	if m.tab != TabLogs {
		t.Errorf("expected TabLogs, got %v", m.tab)
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "4"})
	if m.tab != TabFiles {
		t.Errorf("expected TabFiles, got %v", m.tab)
	}

	// 4. App Filtering
	m.tab = TabApps
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
	m.tab = TabFiles
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

	// 6. Logs
	m.tab = TabLogs
	m.SetLogBundle("com.apple.Maps")
	m.AppendLog("Log line 1")
	m.AppendLog("Log line 2")

	if len(m.logs) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(m.logs))
	}
	if !strings.Contains(m.logsVP.View(), "Log line 1") {
		t.Error("Log content not found in viewport")
	}
}
