package ui

import (
	"strings"
	"testing"
	"time"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

// ── helpers ───────────────────────────────────────────────────────────────

func newPaneWithDevice() MainPane {
	m := newTestPane()
	dev := &device.Device{
		ID:       "test-udid-1234",
		Name:     "iPhone 15",
		Platform: device.PlatformIOS,
		Status:   device.StatusRunning,
		Version:  "17.0",
	}
	m.SetDevice(dev, nil)
	return m
}

func lineCount(s string) int { return len(strings.Split(s, "\n")) }

// ── View ─────────────────────────────────────────────────────────────────

func TestMainPaneView_TooSmall(t *testing.T) {
	m := NewMainPane()
	m.SetSize(5, 3)
	if got := m.View(); got != "" {
		t.Errorf("expected empty for under-minimum size, got %q", got)
	}
}

func TestMainPaneView_NoDevice_HasHint(t *testing.T) {
	m := newTestPane()
	got := m.View()
	if !strings.Contains(got, "load") {
		t.Error("expected hint text in no-device view")
	}
}

func TestMainPaneView_FocusedBorderDiffers(t *testing.T) {
	m := newPaneWithDevice()
	m.SetFocused(false)
	unfocused := m.View()
	m.SetFocused(true)
	focused := m.View()
	if unfocused == focused {
		t.Error("focused and unfocused views should differ (border color)")
	}
}

func TestMainPaneView_FilesTab_BottomJunction(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabFiles
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetTree(root)
	got := m.View()
	// Bottom border with ┴ junction should appear when tree is set.
	if !strings.Contains(got, "┴") {
		t.Error("expected ┴ junction in bottom border for Files tab with tree")
	}
}

// ── renderTitleRow ────────────────────────────────────────────────────────

func TestRenderTitleRow_Running(t *testing.T) {
	m := newPaneWithDevice()
	got := m.renderTitleRow(76)
	if !strings.Contains(got, "iPhone 15") {
		t.Error("device name not in title row")
	}
	// Running device shows green dot (●).
	if !strings.Contains(got, "●") {
		t.Error("expected dot in title row")
	}
}

func TestRenderTitleRow_Offline_ShortID(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{
		ID: "short", Name: "Pixel", Platform: device.PlatformAndroid,
		Status: device.StatusOff,
	}
	m.SetDevice(dev, nil)
	got := m.renderTitleRow(76)
	if !strings.Contains(got, "Pixel") {
		t.Error("device name not in title row")
	}
}

func TestRenderTitleRow_LongID_Truncated(t *testing.T) {
	m := newTestPane()
	dev := &device.Device{
		ID: "12345678-ABCD-EFGH-IJKL-MNOPQRSTUVWX", Name: "iPhone",
		Platform: device.PlatformIOS, Status: device.StatusRunning,
	}
	m.SetDevice(dev, nil)
	got := m.renderTitleRow(76)
	// udid truncated to 8 chars + ellipsis.
	if !strings.Contains(got, "12345678…") {
		t.Error("expected truncated udid with ellipsis")
	}
}

// ── renderTabContent / renderPlaceholder ─────────────────────────────────

func TestRenderTabContent_UnknownTab_ShowsPlaceholder(t *testing.T) {
	m := newPaneWithDevice()
	// Use an out-of-range tab value to hit the default branch.
	m.tab = MainTab(99)
	got := m.renderTabContent(70, 10)
	if !strings.Contains(got, "not implemented") {
		t.Errorf("expected placeholder text, got %q", got)
	}
	if lineCount(got) != 10 {
		t.Errorf("expected %d lines, got %d", 10, lineCount(got))
	}
}

func TestRenderPlaceholder_FillsHeight(t *testing.T) {
	m := newTestPane()
	got := m.renderPlaceholder(60, 8)
	if lineCount(got) != 8 {
		t.Errorf("expected 8 lines, got %d", lineCount(got))
	}
}

// ── padBg ─────────────────────────────────────────────────────────────────

func TestPadBg_ZeroWidth(t *testing.T) {
	if got := padBg(0); got != "" {
		t.Errorf("expected empty for width 0, got %q", got)
	}
}

func TestPadBg_Negative(t *testing.T) {
	if got := padBg(-5); got != "" {
		t.Errorf("expected empty for negative width, got %q", got)
	}
}

func TestPadBg_PositiveWidth(t *testing.T) {
	got := padBg(20)
	if lipgloss.Width(got) != 20 {
		t.Errorf("expected visual width 20, got %d", lipgloss.Width(got))
	}
}

// ── filesLayout ───────────────────────────────────────────────────────────

func TestFilesLayout_NarrowPane(t *testing.T) {
	// Narrow enough that previewW would be < 16 — layout adjusts.
	treeW, sepW, previewW := filesLayout(50)
	if sepW != 1 {
		t.Errorf("sepW: expected 1, got %d", sepW)
	}
	if previewW < 16 {
		t.Errorf("previewW must be ≥16, got %d", previewW)
	}
	if treeW+sepW+previewW != 50 {
		t.Errorf("widths must sum to innerW: %d+%d+%d=%d", treeW, sepW, previewW, treeW+sepW+previewW)
	}
}

func TestFilesLayout_WidePane(t *testing.T) {
	treeW, sepW, previewW := filesLayout(120)
	if treeW+sepW+previewW != 120 {
		t.Errorf("widths must sum to 120, got %d", treeW+sepW+previewW)
	}
}

// ── renderApps ────────────────────────────────────────────────────────────

func TestRenderApps_Empty_ShowsHint(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabApps
	m.SetApps(nil)
	got := m.renderApps(70, 10)
	if !strings.Contains(got, "no apps") {
		t.Errorf("expected 'no apps' hint, got %q", got)
	}
	if lineCount(got) != 10 {
		t.Errorf("expected %d lines, got %d", 10, lineCount(got))
	}
}

func TestRenderApps_NoMatches_ShowsHint(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabApps
	m.SetApps([]device.App{{BundleID: "com.apple.Maps", Name: "Maps"}})
	m.appsFilter = "zzznomatch"
	got := m.renderApps(70, 10)
	if !strings.Contains(got, "no matches") {
		t.Errorf("expected 'no matches' hint, got %q", got)
	}
}

func TestRenderApps_WithApps_ShowsLabels(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabApps
	m.SetApps([]device.App{
		{BundleID: "com.apple.Maps", Name: "Maps", ShortVersion: "3.0"},
		{BundleID: "com.spotify.music", Name: "Spotify", ShortVersion: "8.1"},
	})
	got := m.renderApps(70, 15)
	if !strings.Contains(got, "Maps") {
		t.Error("expected 'Maps' in apps render")
	}
	if !strings.Contains(got, "Spotify") {
		t.Error("expected 'Spotify' in apps render")
	}
}

func TestRenderApps_ScrollOffset(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabApps
	apps := make([]device.App, 20)
	for i := range apps {
		apps[i] = device.App{BundleID: "com.app." + string(rune('a'+i)), Name: string(rune('A' + i))}
	}
	m.SetApps(apps)
	// Force cursor past the visible window (height=5 → listH=4).
	m.appsIdx = 18
	got := m.renderApps(70, 5)
	if lineCount(got) != 5 {
		t.Errorf("expected %d lines, got %d", 5, lineCount(got))
	}
}

// ── renderAppRow ──────────────────────────────────────────────────────────

func TestRenderAppRow_NotSelected(t *testing.T) {
	m := newPaneWithDevice()
	app := device.App{BundleID: "com.example.app", Name: "Example", ShortVersion: "1.0"}
	got := m.renderAppRow(app, 70, false, false)
	if !strings.Contains(got, "Example") {
		t.Error("app name not in row")
	}
	if !strings.Contains(got, "1.0") {
		t.Error("version not in row")
	}
}

func TestRenderAppRow_Selected(t *testing.T) {
	m := newPaneWithDevice()
	app := device.App{BundleID: "com.example", Name: "Example", ShortVersion: "2.0"}
	got := m.renderAppRow(app, 70, true, false)
	if !strings.Contains(got, "Example") {
		t.Error("app name not in selected row")
	}
}

func TestRenderAppRow_Streaming(t *testing.T) {
	m := newPaneWithDevice()
	app := device.App{BundleID: "com.example", Name: "Example", ShortVersion: "2.0"}
	// streaming=true, not selected
	got := m.renderAppRow(app, 70, false, true)
	// Stream icon ⊙ should appear.
	if !strings.Contains(got, "⊙") {
		t.Error("expected stream icon ⊙ in streaming row")
	}
}

func TestRenderAppRow_StreamingSelected(t *testing.T) {
	m := newPaneWithDevice()
	app := device.App{BundleID: "com.example", Name: "Example"}
	got := m.renderAppRow(app, 70, true, true)
	if !strings.Contains(got, "Example") {
		t.Error("app name not in streaming+selected row")
	}
}

func TestRenderAppRow_SystemApp(t *testing.T) {
	m := newPaneWithDevice()
	app := device.App{BundleID: "com.apple.Foo", Name: "Foo", Type: "System"}
	// cursor=true triggers the dim background branch for system apps.
	got := m.renderAppRow(app, 70, true, false)
	if !strings.Contains(got, "Foo") {
		t.Error("system app name not in row")
	}
}

func TestRenderAppRow_FallbackToVersion(t *testing.T) {
	m := newPaneWithDevice()
	// ShortVersion empty, Version set — meta falls back to Version.
	app := device.App{BundleID: "com.ex", Name: "Ex", Version: "3.0"}
	got := m.renderAppRow(app, 70, false, false)
	if !strings.Contains(got, "3.0") {
		t.Error("fallback version not in row")
	}
}

// ── renderInfo ────────────────────────────────────────────────────────────

func TestRenderInfo_Empty_ShowsLoading(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabInfo
	// No info set → shows "loading info…"
	got := m.renderInfo(70, 8)
	if !strings.Contains(got, "loading") {
		t.Errorf("expected loading hint, got %q", got)
	}
	if lineCount(got) != 8 {
		t.Errorf("expected %d lines, got %d", 8, lineCount(got))
	}
}

func TestRenderInfo_WithFields(t *testing.T) {
	m := newPaneWithDevice()
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{
		{Key: "Status", Value: "Running"},
		{Key: "OS", Value: "iOS 17"},
	}})
	got := m.renderInfo(70, 10)
	if !strings.Contains(got, "Status") {
		t.Error("field key 'Status' not in render")
	}
	if !strings.Contains(got, "Running") {
		t.Error("field value 'Running' not in render")
	}
}

func TestRenderInfo_SelectedRow(t *testing.T) {
	m := newPaneWithDevice()
	m.SetInfo(device.DeviceInfo{Fields: []device.InfoField{
		{Key: "Name", Value: "iPhone"},
		{Key: "OS", Value: "iOS"},
	}})
	m.infoIdx = 1
	got := m.renderInfo(70, 5)
	if !strings.Contains(got, "OS") {
		t.Error("selected field not rendered")
	}
}

func TestRenderInfo_ScrollOffset(t *testing.T) {
	m := newPaneWithDevice()
	fields := make([]device.InfoField, 20)
	for i := range fields {
		fields[i] = device.InfoField{Key: "k", Value: "v"}
	}
	m.SetInfo(device.DeviceInfo{Fields: fields})
	m.infoIdx = 18
	got := m.renderInfo(70, 5)
	if lineCount(got) != 5 {
		t.Errorf("expected %d lines, got %d", 5, lineCount(got))
	}
}

// ── renderInfoRow ─────────────────────────────────────────────────────────

func TestRenderInfoRow_NotSelected(t *testing.T) {
	got := renderInfoRow("Status", "Running", 8, 70, false)
	if !strings.Contains(got, "Status") || !strings.Contains(got, "Running") {
		t.Error("key or value missing from unselected info row")
	}
}

func TestRenderInfoRow_Selected(t *testing.T) {
	got := renderInfoRow("Status", "Running", 8, 70, true)
	if !strings.Contains(got, "Status") {
		t.Error("key missing from selected info row")
	}
}

func TestRenderInfoRow_LongValue_Truncated(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := renderInfoRow("Key", long, 6, 70, false)
	if lipgloss.Width(got) > 70 {
		t.Errorf("row exceeds width: %d", lipgloss.Width(got))
	}
}

func TestRenderInfoRow_LongKey_Truncated(t *testing.T) {
	got := renderInfoRow(strings.Repeat("k", 30), "val", 14, 70, false)
	if lipgloss.Width(got) > 70 {
		t.Errorf("row exceeds width: %d", lipgloss.Width(got))
	}
}

// ── infoKeyWidth ─────────────────────────────────────────────────────────

func TestInfoKeyWidth_Capped(t *testing.T) {
	fields := []device.InfoField{{Key: strings.Repeat("X", 30), Value: "v"}}
	w := infoKeyWidth(fields)
	if w > 14 {
		t.Errorf("expected cap of 14, got %d", w)
	}
}

func TestInfoKeyWidth_MinFour(t *testing.T) {
	w := infoKeyWidth([]device.InfoField{{Key: "ab", Value: "v"}})
	if w < 4 {
		t.Errorf("expected min of 4, got %d", w)
	}
}

// ── renderLogs ────────────────────────────────────────────────────────────

func TestRenderLogs_NoBundle_ShowsHint(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabLogs
	// logBundle is empty by default after SetDevice.
	got := m.renderLogs(70, 10)
	if !strings.Contains(got, "no selected app") {
		t.Errorf("expected no-bundle hint, got %q", got)
	}
	if lineCount(got) != 10 {
		t.Errorf("expected %d lines, got %d", 10, lineCount(got))
	}
}

func TestRenderLogs_WithBundle_ShowsHeader(t *testing.T) {
	m := newPaneWithDevice()
	m.tab = TabLogs
	m.SetLogBundle("com.example.app")
	m.AppendLog("first line")
	m.AppendLog("second line")
	// Use View() which correctly accounts for viewport height vs content height.
	got := m.View()
	if !strings.Contains(got, "com.example.app") {
		t.Error("bundle name not in logs view")
	}
	if !strings.Contains(got, "streaming") {
		t.Error("'streaming' header not in logs view")
	}
}

// ── renderPreviewPane ─────────────────────────────────────────────────────

func TestRenderPreviewPane_NoSelection(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true}
	m.SetTree(root)
	// No children → flattenTree is empty → sel is nil.
	lines := m.renderPreviewPane(30, 10)
	if len(lines) != 10 {
		t.Errorf("expected %d lines, got %d", 10, len(lines))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "preview") {
		t.Error("expected 'preview' label when no node selected")
	}
}

func TestRenderPreviewPane_FileSelected(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{
		Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/data.json", Name: "data.json", IsDir: false, Size: 1024,
				Modified: time.Now(), Permissions: "-rw-r--r--"},
		},
	}
	m.SetTree(root)
	m.expanded["/"] = true
	m.treeIdx = 0 // points to data.json in flattenTree
	lines := m.renderPreviewPane(30, 10)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "data.json") {
		t.Error("selected file name not in preview")
	}
	if !strings.Contains(joined, "size") {
		t.Error("expected 'size' field in file preview")
	}
}

func TestRenderPreviewPane_DirSelected(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{
		Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/subdir", Name: "subdir", IsDir: true, Modified: time.Now()},
		},
	}
	m.SetTree(root)
	m.expanded["/"] = true
	m.treeIdx = 0
	lines := m.renderPreviewPane(30, 10)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "directory") {
		t.Error("expected 'directory' type in dir preview")
	}
}

func TestRenderPreviewPane_SQLiteFile_ShowsHint(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{
		Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/app.db", Name: "app.db", IsDir: false, Size: 4096, Modified: time.Now()},
		},
	}
	m.SetTree(root)
	m.expanded["/"] = true
	m.treeIdx = 0
	lines := m.renderPreviewPane(40, 15)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "SQLite") {
		t.Error("expected SQLite label in preview")
	}
	if !strings.Contains(joined, "viewer") {
		t.Error("expected SQLite viewer hint")
	}
}

// ── previewFields ─────────────────────────────────────────────────────────

func TestPreviewFields_Directory(t *testing.T) {
	n := &device.FileNode{IsDir: true, Children: []device.FileNode{{}, {}}, Modified: time.Now()}
	fields := previewFields(n)
	keys := make(map[string]bool)
	for _, f := range fields {
		keys[f.k] = true
	}
	if !keys["type"] || !keys["items"] || !keys["modified"] {
		t.Errorf("missing expected fields for directory: %v", fields)
	}
}

func TestPreviewFields_File(t *testing.T) {
	n := &device.FileNode{IsDir: false, Name: "report.pdf", Size: 512, Modified: time.Now()}
	fields := previewFields(n)
	keys := make(map[string]bool)
	for _, f := range fields {
		keys[f.k] = true
	}
	if !keys["type"] || !keys["size"] {
		t.Errorf("missing expected fields for file: %v", fields)
	}
}

// ── previewContent ────────────────────────────────────────────────────────

func TestPreviewContent_Directory(t *testing.T) {
	n := &device.FileNode{IsDir: true, Children: []device.FileNode{{}, {}, {}}}
	lines := previewContent(n)
	if len(lines) == 0 || !strings.Contains(lines[0], "directory") {
		t.Errorf("expected directory content, got %v", lines)
	}
	if !strings.Contains(lines[0], "3") {
		t.Error("expected entry count in directory content")
	}
}

func TestPreviewContent_SQLite(t *testing.T) {
	n := &device.FileNode{Name: "cache.db", Size: 8192}
	lines := previewContent(n)
	if len(lines) == 0 || !strings.Contains(lines[0], "SQLite") {
		t.Errorf("expected SQLite content, got %v", lines)
	}
}

func TestPreviewContent_LogFile(t *testing.T) {
	n := &device.FileNode{Name: "app.log", Size: 2048}
	lines := previewContent(n)
	if len(lines) == 0 || !strings.Contains(lines[0], "text") {
		t.Errorf("expected text content for .log file, got %v", lines)
	}
}

func TestPreviewContent_JSONFile(t *testing.T) {
	n := &device.FileNode{Name: "config.json", Size: 512}
	lines := previewContent(n)
	if !strings.Contains(lines[0], "text") {
		t.Errorf("expected text content for .json, got %v", lines)
	}
}

func TestPreviewContent_BinaryFile(t *testing.T) {
	n := &device.FileNode{Name: "app.bin", Size: 1024}
	lines := previewContent(n)
	if len(lines) == 0 || !strings.Contains(lines[0], "binary") {
		t.Errorf("expected binary content, got %v", lines)
	}
}

// ── renderField ───────────────────────────────────────────────────────────

func TestRenderField_Basic(t *testing.T) {
	got := renderField("type", "file (.json)", 40)
	if !strings.Contains(got, "type") {
		t.Error("key 'type' not in field render")
	}
	if !strings.Contains(got, ".json") {
		t.Error("value not in field render")
	}
}

func TestRenderField_LongKey_Truncated(t *testing.T) {
	got := renderField(strings.Repeat("k", 30), "v", 40)
	if lipgloss.Width(got) > 40 {
		t.Errorf("field exceeds width: %d", lipgloss.Width(got))
	}
}

func TestRenderField_LongValue_Truncated(t *testing.T) {
	got := renderField("key", strings.Repeat("v", 100), 40)
	if lipgloss.Width(got) > 40 {
		t.Errorf("field exceeds width: %d", lipgloss.Width(got))
	}
}

// ── renderTreeRow ─────────────────────────────────────────────────────────

func TestRenderTreeRow_File_NotSelected(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{{Path: "/foo.txt", Name: "foo.txt", IsDir: false, Size: 512}},
	}
	m.SetTree(root)
	rows := m.flattenTree()
	if len(rows) == 0 {
		t.Fatal("expected at least one tree row")
	}
	got := m.renderTreeRow(rows[0], 60, false)
	if !strings.Contains(got, "foo.txt") {
		t.Error("file name not in tree row")
	}
	// size shown for files
	if !strings.Contains(got, "B") {
		t.Error("expected size in file row")
	}
}

func TestRenderTreeRow_Dir_Expanded(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/subdir", Name: "subdir", IsDir: true,
				Children: []device.FileNode{{Path: "/subdir/f", Name: "f"}}},
		},
	}
	m.SetTree(root)
	m.expanded["/subdir"] = true
	rows := m.flattenTree()
	got := m.renderTreeRow(rows[0], 60, false)
	// Expanded dir gets ▾ caret.
	if !strings.Contains(got, "▾") {
		t.Error("expected ▾ for expanded directory")
	}
}

func TestRenderTreeRow_Dir_Collapsed(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/subdir", Name: "subdir", IsDir: true,
				Children: []device.FileNode{{Path: "/subdir/f", Name: "f"}}},
		},
	}
	m.SetTree(root)
	// SetTree auto-expands direct children; collapse /subdir explicitly.
	delete(m.expanded, "/subdir")
	rows := m.flattenTree()
	got := m.renderTreeRow(rows[0], 60, false)
	if !strings.Contains(got, "▸") {
		t.Error("expected ▸ for collapsed directory")
	}
}

func TestRenderTreeRow_Selected(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{{Path: "/a", Name: "a", IsDir: false, Size: 10}},
	}
	m.SetTree(root)
	rows := m.flattenTree()
	selected := m.renderTreeRow(rows[0], 60, true)
	notSelected := m.renderTreeRow(rows[0], 60, false)
	if selected == notSelected {
		t.Error("selected and unselected tree rows should differ")
	}
}

func TestRenderTreeRow_EmptyDir(t *testing.T) {
	m := newPaneWithDevice()
	root := &device.FileNode{Path: "/", Name: "/", IsDir: true,
		Children: []device.FileNode{
			{Path: "/empty", Name: "empty", IsDir: true},
		},
	}
	m.SetTree(root)
	rows := m.flattenTree()
	got := m.renderTreeRow(rows[0], 60, false)
	// Empty dir shows "—" as meta.
	if !strings.Contains(got, "—") {
		t.Error("expected — for empty directory")
	}
}

// ── formatSize ────────────────────────────────────────────────────────────

func TestFormatSize(t *testing.T) {
	tests := []struct{ b int64; want string }{
		{500, "500 B"},
		{1500, "1 KB"},
		{1_500_000, "1.5 MB"},
		{2_500_000_000, "2.5 GB"},
	}
	for _, tc := range tests {
		got := formatSize(tc.b)
		if got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.b, got, tc.want)
		}
	}
}

// ── fileExtSuffix ─────────────────────────────────────────────────────────

func TestFileExtSuffix(t *testing.T) {
	tests := []struct{ name, want string }{
		{"file.json", "(.json)"},
		{"no_extension", ""},
		{"archive.tar.gz", "(.gz)"},
	}
	for _, tc := range tests {
		got := fileExtSuffix(tc.name)
		if got != tc.want {
			t.Errorf("fileExtSuffix(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
