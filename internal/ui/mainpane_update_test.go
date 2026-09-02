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
	m.panel = panelApps
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
	m.panel = panelApps
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
	m.panel = panelApps
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
	m.panel = panelApps
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
	m.panel = panelApps
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
	m.panel = panelApps
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
	m.panel = panelApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})

	_, cmd := m.Update(tea.KeyPressMsg{Text: "a"})
	if cmd == nil {
		t.Fatal("expected command from a key")
	}
	msg := cmd()
	if _, ok := msg.(ShowInstallAppMsg); !ok {
		t.Errorf("expected ShowInstallAppMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_TabApps_InstallKey_NoDevice(t *testing.T) {
	m := newTestPane()
	m.panel = panelApps
	// No device set.
	_, cmd := m.Update(tea.KeyPressMsg{Text: "a"})
	if cmd != nil {
		t.Error("expected no command when no device")
	}
}

func TestMainPaneUpdate_TabApps_RebuildKey(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning, Kind: device.KindVirtual}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A", Type: "User"}})
	m.appsIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Text: "r"})
	if cmd == nil {
		t.Fatal("expected command from r key")
	}
	msg := cmd()
	req, ok := msg.(RequestRebuildMsg)
	if !ok {
		t.Fatalf("expected RequestRebuildMsg, got %T", msg)
	}
	if req.App.BundleID != "com.a" {
		t.Errorf("expected app com.a, got %s", req.App.BundleID)
	}
}

func TestMainPaneUpdate_TabApps_RebuildKey_NoDevice(t *testing.T) {
	m := newTestPane()
	m.panel = panelApps
	_, cmd := m.Update(tea.KeyPressMsg{Text: "r"})
	if cmd != nil {
		t.Error("expected no command when no device")
	}
}

func TestMainPaneUpdate_TabApps_RebuildKey_EmptyList(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.SetApps(nil)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "r"})
	if cmd != nil {
		t.Error("expected no command for empty apps list")
	}
}

func TestMainPaneUpdate_TabApps_DeleteKey_UserApp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
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
	m.panel = panelApps
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
	m.panel = panelApps
	m.SetApps(nil)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "d"})
	if cmd != nil {
		t.Error("expected no command for empty apps list")
	}
}

func TestMainPaneUpdate_TabApps_Space_PinsApp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.appsIdx = 0

	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m2.selectedApp == nil || m2.selectedApp.BundleID != "com.a" {
		t.Fatalf("expected com.a pinned, got %v", m2.selectedApp)
	}
}

func TestMainPaneUpdate_TabApps_Space_UnpinsApp(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	app := device.App{BundleID: "com.a", Name: "A"}
	m.SetApps([]device.App{app})
	m.appsIdx = 0
	m.selectedApp = &app // already pinned

	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if m2.selectedApp != nil {
		t.Error("expected selectedApp cleared after unpin")
	}
}

func TestMainPaneUpdate_TabApps_L_StartsLogging(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.appsIdx = 0

	_, cmd := m.Update(tea.KeyPressMsg{Text: "l"})
	if cmd == nil {
		t.Fatal("expected command from 'l' key")
	}
	req, ok := cmd().(StartAppLoggingMsg)
	if !ok {
		t.Fatalf("expected StartAppLoggingMsg, got %T", cmd())
	}
	if req.App.BundleID != "com.a" {
		t.Errorf("wrong bundle ID: %q", req.App.BundleID)
	}
}

func TestMainPaneUpdate_TabApps_L_StopsLogging(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.SetApps([]device.App{{BundleID: "com.a", Name: "A"}})
	m.appsIdx = 0
	m.SetLoggingBundle("com.a") // already logging this app

	_, cmd := m.Update(tea.KeyPressMsg{Text: "l"})
	if cmd == nil {
		t.Fatal("expected command")
	}
	if _, ok := cmd().(StopAppLoggingMsg); !ok {
		t.Errorf("expected StopAppLoggingMsg, got %T", cmd())
	}
}

func TestMainPaneUpdate_TabApps_Space_EmptyList(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
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
	m.panel = panelApps
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

// ── esc unwind chain ─────────────────────────────────────────────────────

func TestMainPaneUpdate_Esc_ClearsFilterThenCollapsesThenReleases(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelApps
	m.appsFilter = "foo"

	// 1st esc clears the filter.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.appsFilter != "" || m.panel != panelApps {
		t.Fatalf("expected filter cleared, panel unchanged; got filter=%q panel=%v", m.appsFilter, m.panel)
	}
	// 2nd esc collapses Apps back to Files.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.panel != panelFiles {
		t.Fatalf("expected panelFiles after collapse, got %v", m.panel)
	}
	// 3rd esc releases focus to the sidebar.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected ReleaseFocusMsg cmd")
	}
	if _, ok := cmd().(ReleaseFocusMsg); !ok {
		t.Errorf("expected ReleaseFocusMsg, got %T", cmd())
	}
}

// ── TabFiles enter ─────────────────────────────────────────────────────────

func TestMainPaneUpdate_TabFiles_Enter_ToggleDir(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	m.panel = panelFiles
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
	m.panel = panelFiles
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
	m.panel = panelFiles
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
	m.panel = panelFiles
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
	m.panel = panelFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetTree(root)
	m.treeIdx = 99 // out of range

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected no command for out-of-range tree index")
	}
}

// ── Tab 4 with no tree ─────────────────────────────────────────────────────

func TestMainPaneUpdate_Tab3_NoTree_RequestsFileTree(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	m.SetDevice(dev, nil)
	// tree is nil after SetDevice(dev, nil)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "3"})
	if cmd == nil {
		t.Fatal("expected RequestFileTreeMsg command")
	}
	msg := cmd()
	if _, ok := msg.(RequestFileTreeMsg); !ok {
		t.Errorf("expected RequestFileTreeMsg, got %T", msg)
	}
}

func TestMainPaneUpdate_Tab3_WithTree_NoRequest(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning}
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetDevice(dev, root)

	_, cmd := m.Update(tea.KeyPressMsg{Text: "3"})
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
	m.panel = panelFiles

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
	got := m.renderAppsFilterBar()
	// Active cursor block should appear.
	if !strings.Contains(got, "map") {
		t.Error("expected filter text in bar")
	}
}
