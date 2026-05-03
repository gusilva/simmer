package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

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
