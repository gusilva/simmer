package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

// ── filter input mode ─────────────────────────────────────────────────────

func TestMainPaneUpdate_FilterMode_Backspace(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.appsFiltering = true
	m.appsFilter = "ab"

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.appsFilter != "a" {
		t.Errorf("expected 'a' after backspace, got %q", m.appsFilter)
	}
}

func TestMainPaneUpdate_FilterMode_BackspaceEmpty(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.appsFiltering = true
	m.appsFilter = ""
	// Backspace on empty filter should not panic or change anything.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.appsFilter != "" {
		t.Errorf("expected empty filter, got %q", m.appsFilter)
	}
}

func TestMainPaneUpdate_FilterMode_Enter_ExitsFilter(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.appsFiltering = true

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.appsFiltering {
		t.Error("expected appsFiltering false after enter")
	}
}

func TestMainPaneUpdate_FilterMode_Esc_ExitsFilter(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.appsFiltering = true

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.appsFiltering {
		t.Error("expected appsFiltering false after esc")
	}
}

func TestMainPaneUpdate_FilterMode_Text_Appends(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.appsFiltering = true
	m.appsFilter = "x"

	m, _ = m.Update(tea.KeyPressMsg{Text: "y"})
	if m.appsFilter != "xy" {
		t.Errorf("expected 'xy', got %q", m.appsFilter)
	}
}

// ── TabApps navigation ────────────────────────────────────────────────────

func TestMainPaneUpdate_TabApps_HomeEnd(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{
		{BundleID: "com.a", Name: "A"},
		{BundleID: "com.b", Name: "B"},
		{BundleID: "com.c", Name: "C"},
	})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'G'})
	if m.appsIdx != 2 {
		t.Errorf("expected appsIdx 2 after G, got %d", m.appsIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g'})
	if m.appsIdx != 0 {
		t.Errorf("expected appsIdx 0 after g, got %d", m.appsIdx)
	}
}

func TestMainPaneUpdate_TabApps_InstallKey(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})

	_, cmd := m.Update(tea.KeyPressMsg{Text: "i"})
	if cmd == nil {
		t.Fatal("expected command from i key")
	}
	msg := cmd()
	if _, ok := msg.(ShowInstallAppMsg); !ok {
		t.Errorf("expected ShowInstallAppMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_TabApps_InstallKey_NoDevice(t *testing.T) {
	m := newTestPane()
	m.tab = TabApps
	// No device set.
	_, cmd := m.Update(tea.KeyPressMsg{Text: "i"})
	if cmd != nil {
		t.Error("expected no command when no device")
	}
}

func TestMainPaneUpdate_TabApps_DeleteKey_UserApp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A", Type: "User"}})
	m.appsIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Text: "d"})
	if cmd == nil {
		t.Fatal("expected command from d key")
	}
	msg := cmd()
	if _, ok := msg.(ShowDeleteAppMsg); !ok {
		t.Errorf("expected ShowDeleteAppMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_TabApps_DeleteKey_SystemApp_NoOp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{{BundleID: "com.apple.Foo", Name: "Foo", Type: "System"}})
	m.appsIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Text: "d"})
	if cmd != nil {
		t.Error("expected no command for system app delete")
	}
}

func TestMainPaneUpdate_TabApps_DeleteKey_EmptyList(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps(nil)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "d"})
	if cmd != nil {
		t.Error("expected no command for empty apps list")
	}
}

func TestMainPaneUpdate_TabApps_Space_StartsStream(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.appsIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected command from space key")
	}
	msg := cmd()
	req, ok := msg.(RequestLogStreamMsg)
	if !ok {
		t.Fatalf("expected RequestLogStreamMsg, got %T", msg)
	}
	if req.App.BundleID != "com.a" {
		t.Errorf("wrong bundle ID: %q", req.App.BundleID)
	}
}

func TestMainPaneUpdate_TabApps_Space_StopsStream(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	app := device.App{BundleID: "com.a", Name: "A"}
	m.SetApps([]device.App{app})
	m.appsIdx = 0
	m.selectedApp = &app // already streaming this app

	m2, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(StopLogStreamMsg); !ok {
		t.Errorf("expected StopLogStreamMsg, got %T", msg)
	}
	if m2.selectedApp != nil {
		t.Error("expected selectedApp cleared after stopping stream")
	}
}

func TestMainPaneUpdate_TabApps_Space_EmptyList(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps(nil)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd != nil {
		t.Error("expected no command for space on empty list")
	}
}

func TestMainPaneUpdate_TabApps_Navigation_EmitsAppFocused(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabApps
	m.SetApps([]device.App{
		{BundleID: "com.a", Name: "A"},
		{BundleID: "com.b", Name: "B"},
	})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd == nil {
		t.Fatal("expected AppFocusedMsg cmd")
	}
	msg := cmd()
	if _, ok := msg.(AppFocusedMsg); !ok {
		t.Errorf("expected AppFocusedMsg, got %T", msg)
	}
}

// ── TabInfo ────────────────────────────────────────────────────────────────

func TestMainPaneUpdate_TabInfo_HomeEnd(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabInfo
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{
		{Key: "A", Value: "1"}, {Key: "B", Value: "2"}, {Key: "C", Value: "3"},
	}})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'G'})
	if m.infoIdx != 2 {
		t.Errorf("expected infoIdx 2 after G, got %d", m.infoIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g'})
	if m.infoIdx != 0 {
		t.Errorf("expected infoIdx 0 after g, got %d", m.infoIdx)
	}
}

func TestMainPaneUpdate_TabInfo_Space_CopiesValue(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabInfo
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{{Key: "ID", Value: "test-uuid"}}})
	m.infoIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected clipboard command from space on info")
	}
}

func TestMainPaneUpdate_TabInfo_Space_OutOfRange(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabInfo
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{{Key: "k", Value: "v"}}})
	m.infoIdx = 5 // out of range

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd != nil {
		t.Error("expected no command when infoIdx out of range")
	}
}

// ── TabFiles enter ─────────────────────────────────────────────────────────

func TestMainPaneUpdate_TabFiles_Enter_ToggleDir(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/sub", Name: "sub", IsDir: true,
				Children: []device.FileNode{{Path: "/sub/f", Name: "f"}}},
		},
	}
	m.SetTree(root)
	delete(m.expanded, "/sub") // ensure collapsed
	m.treeIdx = 0              // on /sub

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.expanded["/sub"] {
		t.Error("expected /sub expanded after enter")
	}
}

func TestMainPaneUpdate_TabFiles_Enter_NonDB_File_NoOp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{{Path: "/notes.txt", Name: "notes.txt", IsDir: false}},
	}
	m.SetTree(root)
	m.treeIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for non-db file")
	}
}

func TestMainPaneUpdate_TabFiles_Enter_DBFile_OpensSQLite(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{{Path: "/data.db", Name: "data.db", IsDir: false}},
	}
	m.SetTree(root)
	m.treeIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command for .db file")
	}
	msg := cmd()
	if _, ok := msg.(ShowSQLiteViewerMsg); !ok {
		t.Errorf("expected ShowSQLiteViewerMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_TabFiles_Enter_SQLiteFile_OpensSQLite(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{{Path: "/store.sqlite", Name: "store.sqlite", IsDir: false}},
	}
	m.SetTree(root)
	m.treeIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command for .sqlite file")
	}
	msg := cmd()
	if _, ok := msg.(ShowSQLiteViewerMsg); !ok {
		t.Errorf("expected ShowSQLiteViewerMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_TabFiles_Enter_OutOfRange(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.tab = TabFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetTree(root)
	m.treeIdx = 99 // out of range

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for out-of-range tree index")
	}
}

// ── Tab 4 with no tree ─────────────────────────────────────────────────────

func TestMainPaneUpdate_Tab4_NoTree_RequestsFileTree(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	// tree is nil after SetDevice(dev, nil)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "4"})
	if cmd == nil {
		t.Fatal("expected RequestFileTreeMsg command")
	}
	msg := cmd()
	if _, ok := msg.(RequestFileTreeMsg); !ok {
		t.Errorf("expected RequestFileTreeMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_Tab4_WithTree_NoRequest(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetDevice(dev, root)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "4"})
	if cmd != nil {
		t.Error("expected no command when tree already loaded")
	}
}

// ── Tab 2 emits AppFocusedMsg when app selected ───────────────────────────

func TestMainPaneUpdate_Tab2_WithSelectedApp_EmitsFocused(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.tab = TabInfo // start on Info

	_, cmd := m.Update(tea.KeyPressMsg{Text: "2"})
	if cmd == nil {
		t.Fatal("expected AppFocusedMsg cmd when switching to apps tab with an app selected")
	}
	msg := cmd()
	if _, ok := msg.(AppFocusedMsg); !ok {
		t.Errorf("expected AppFocusedMsg, got %T", msg)
	}
}

// ── Non-key message is a no-op ────────────────────────────────────────────

func TestMainPaneUpdate_NonKey_NoOp(t *testing.T) {
	m := newTestPane()
	m2, cmd := m.Update("not a key")
	if cmd != nil {
		t.Error("expected no command for non-key message")
	}
	_ = m2
}

// ── renderAppsFilterBar with cursor active ────────────────────────────────

func TestRenderAppsFilterBar_FilteringActive(t *testing.T) {
	m := newTestPane()
	m.appsFiltering = true
	m.appsFilter = "map"
	got := m.renderAppsFilterBar(60)
	// Active cursor block should appear.
	if !strings.Contains(got, "map") {
		t.Error("expected filter text in bar")
	}
}
