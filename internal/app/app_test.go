package app

import (
	"testing"

	"simmer/internal/device"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"

	tea "charm.land/bubbletea/v2"
)

func TestErrPreview_Nil(t *testing.T) {
	if got := errPreview(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestErrPreview_Short(t *testing.T) {
	err := errorString("connection refused")
	got := errPreview(err)
	if got != "connection refused" {
		t.Errorf("got %q", got)
	}
}

func TestErrPreview_Long(t *testing.T) {
	// Build a string longer than 60 runes.
	long := "this is a very long error message that exceeds the preview limit by quite a lot"
	err := errorString(long)
	got := errPreview(err)
	if len([]rune(got)) > 60 {
		t.Errorf("expected ≤60 runes, got %d: %q", len([]rune(got)), got)
	}
	if got[len(got)-3:] != "…" {
		// last rune should be ellipsis
		runes := []rune(got)
		if string(runes[len(runes)-1]) != "…" {
			t.Errorf("expected ellipsis suffix, got %q", got)
		}
	}
}

func TestTextPreview_Short(t *testing.T) {
	got := textPreview("hello", 10)
	if got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestTextPreview_ExactLimit(t *testing.T) {
	s := "1234567890"
	got := textPreview(s, 10)
	if got != s {
		t.Errorf("got %q, want %q", got, s)
	}
}

func TestTextPreview_Truncated(t *testing.T) {
	s := "abcdefghij"
	got := textPreview(s, 5)
	runes := []rune(got)
	if len(runes) > 5 {
		t.Errorf("exceeds limit: %q", got)
	}
	if string(runes[len(runes)-1]) != "…" {
		t.Errorf("expected ellipsis, got %q", got)
	}
}

func TestTextPreview_NewlinesCollapsed(t *testing.T) {
	got := textPreview("line1\nline2", 20)
	for _, c := range got {
		if c == '\n' {
			t.Errorf("newline not collapsed: %q", got)
		}
	}
}

// errorString implements error for testing.
type errorString string

func (e errorString) Error() string { return string(e) }

// ---------- Update message routing ----------

func newTestModel() model {
	return model{
		sidebar:      ui.NewSidebar(),
		mainPane:     ui.NewMainPane(),
		layout:       &appLayout{},
		toolVersions: make(map[device.Platform]string),
		rebuildPaths: make(map[string]string),
	}
}

func TestUpdate_ClearStatusMsg_MatchingSeq(t *testing.T) {
	m := newTestModel()
	m.status = "hello"
	m.statusSeq = 3
	result, _ := m.Update(clearStatusMsg(3))
	m2 := result.(model)
	if m2.status != "" {
		t.Errorf("expected status cleared, got %q", m2.status)
	}
}

func TestUpdate_ClearStatusMsg_StaleSeq(t *testing.T) {
	m := newTestModel()
	m.status = "hello"
	m.statusSeq = 3
	result, _ := m.Update(clearStatusMsg(2))
	m2 := result.(model)
	if m2.status != "hello" {
		t.Errorf("expected status preserved, got %q", m2.status)
	}
}

func TestUpdate_CancelOverlay_ClearsAllOverlays(t *testing.T) {
	m := newTestModel()
	// Place a non-nil overlay.
	alert := ui.NewDeleteSimulatorAlert(device.Device{Name: "test"})
	m.deleteAlert = &alert
	result, _ := m.Update(ui.CancelOverlayMsg{})
	m2 := result.(model)
	if m2.deleteAlert != nil || m2.platformPicker != nil || m2.createIOSModal != nil || m2.createAndModal != nil || m2.dbViewerModal != nil {
		t.Error("expected all overlays nil after CancelOverlayMsg")
	}
}

func TestUpdate_DiscoveryMsg_UpdatesDeviceCount(t *testing.T) {
	m := newTestModel()
	devices := []device.Device{
		{ID: "1", Platform: device.PlatformIOS, Status: device.StatusRunning},
		{ID: "2", Platform: device.PlatformIOS, Status: device.StatusOff},
		{ID: "3", Platform: device.PlatformAndroid, Status: device.StatusRunning},
	}
	msg := discoveryMsg{Devices: devices}
	result, _ := m.Update(msg)
	m2 := result.(model)
	if m2.bootedCount != 2 {
		t.Errorf("bootedCount: got %d, want 2", m2.bootedCount)
	}
	if m2.iosCount != 2 {
		t.Errorf("iosCount: got %d, want 2", m2.iosCount)
	}
	if m2.androidCount != 1 {
		t.Errorf("androidCount: got %d, want 1", m2.androidCount)
	}
}

func TestUpdate_DiscoveryMsg_ClearsLoading(t *testing.T) {
	m := newTestModel()
	m.loading = true
	result, _ := m.Update(discoveryMsg{})
	m2 := result.(model)
	if m2.loading {
		t.Error("expected loading=false after discoveryMsg")
	}
}

func TestUpdate_WindowSize_StoresSize(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m2 := result.(model)
	if m2.width != 120 || m2.height != 40 {
		t.Errorf("got %dx%d, want 120x40", m2.width, m2.height)
	}
}

func TestUpdate_KeyCtrlC_Quits(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m2 := result.(model)
	if !m2.quitting {
		t.Error("expected quitting=true after ctrl+c")
	}
}

func TestUpdate_KeyQ_Quits(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(tea.KeyPressMsg{Code: 'q'})
	m2 := result.(model)
	if !m2.quitting {
		t.Error("expected quitting=true after q")
	}
}

func TestUpdate_KeyCtrlD_OpensDBViewer(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	m2 := result.(model)
	if m2.dbViewerModal == nil {
		t.Error("expected dbViewerModal to be set after ctrl+d")
	}
}

func TestUpdate_KeyCtrlD_ClosesDBViewer(t *testing.T) {
	m := newTestModel()
	modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
	m.dbViewerModal = &modal
	result, _ := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	m2 := result.(model)
	if m2.dbViewerModal != nil {
		t.Error("expected dbViewerModal to be nil after ctrl+d toggle")
	}
}

func TestUpdate_BootResultMsg_Success(t *testing.T) {
	m := newTestModel()
	dev := device.Device{Name: "iPhone", Platform: device.PlatformIOS}
	result, _ := m.Update(bootResultMsg{device: dev})
	m2 := result.(model)
	if !m2.loading {
		t.Error("expected loading=true after successful boot")
	}
}

func TestUpdate_BootResultMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(bootResultMsg{err: errorString("boot failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_ShutdownResultMsg_Success(t *testing.T) {
	m := newTestModel()
	dev := device.Device{Name: "iPhone"}
	result, _ := m.Update(shutdownResultMsg{device: dev})
	m2 := result.(model)
	if !m2.loading {
		t.Error("expected loading=true after shutdown")
	}
}

func TestUpdate_ShutdownResultMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(shutdownResultMsg{err: errorString("shutdown failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_FileTreeMsg_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(fileTreeMsg{root: device.FileNode{Name: "root"}})
	m2 := result.(model)
	if len(m2.errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestUpdate_FileTreeMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(fileTreeMsg{err: errorString("load failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_AppsListMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(appsListMsg{err: errorString("apps failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_InfoMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(infoMsg{err: errorString("info failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_ClipboardCopiedMsg_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(ui.ClipboardCopiedMsg{Text: "hello"})
	m2 := result.(model)
	if m2.status == "" {
		t.Error("expected status set after clipboard copy")
	}
}

func TestUpdate_ClipboardCopiedMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(ui.ClipboardCopiedMsg{Err: errorString("copy failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_AppFocusedMsg_SetsStatus(t *testing.T) {
	m := newTestModel()
	app := device.App{BundleID: "com.example.app", Name: "MyApp"}
	result, _ := m.Update(ui.AppFocusedMsg{App: app})
	m2 := result.(model)
	if m2.status == "" {
		t.Error("expected status set after app focused")
	}
}

func TestUpdate_LogBatchMsg_NonMatchingBundleID(t *testing.T) {
	m := newTestModel()
	m.logBundleID = "com.other"
	result, _ := m.Update(logBatchMsg{bundleID: "com.example", lines: []string{"log line"}})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_LogEndedMsg_NonMatchingBundleID(t *testing.T) {
	m := newTestModel()
	m.logBundleID = "com.other"
	result, _ := m.Update(logEndedMsg{bundleID: "com.example"})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_RequestRebuildMsg_SystemApp_NoOp(t *testing.T) {
	m := newTestModel()
	dev := device.Device{ID: "1", Platform: device.PlatformIOS, Kind: device.KindVirtual}
	app := device.App{BundleID: "com.apple.Foo", Type: "System"}
	result, cmd := m.Update(ui.RequestRebuildMsg{Device: dev, App: app})
	m2 := result.(model)
	if m2.installAppModal != nil {
		t.Error("expected no install modal for system app rebuild")
	}
	if cmd == nil {
		t.Fatal("expected status cmd")
	}
	if m2.status != "Not a local build" {
		t.Errorf("expected status message, got %q", m2.status)
	}
}

func TestUpdate_RequestRebuildMsg_PhysicalDevice_NoOp(t *testing.T) {
	m := newTestModel()
	dev := device.Device{ID: "1", Platform: device.PlatformIOS, Kind: device.KindPhysical}
	app := device.App{BundleID: "com.example.app", Type: "User"}
	result, _ := m.Update(ui.RequestRebuildMsg{Device: dev, App: app})
	m2 := result.(model)
	if m2.status != "Not a local build" {
		t.Errorf("expected status message, got %q", m2.status)
	}
}

func TestUpdate_RequestRebuildMsg_Android_OpensModalOnFirstRebuild(t *testing.T) {
	m := newTestModel()
	dev := device.Device{ID: "1", Platform: device.PlatformAndroid, Kind: device.KindVirtual}
	app := device.App{BundleID: "com.example.app", Type: "User"}
	result, cmd := m.Update(ui.RequestRebuildMsg{Device: dev, App: app})
	m2 := result.(model)
	if m2.installAppModal == nil {
		t.Error("expected install modal opened to pick Android project path")
	}
	if cmd == nil {
		t.Error("expected focus cmd")
	}
}

func TestUpdate_RequestRebuildMsg_Android_UsesCachedPath(t *testing.T) {
	m := newTestModel()
	m.rebuildPaths["com.example.app"] = "/some/project/dir"
	dev := device.Device{ID: "1", Platform: device.PlatformAndroid, Kind: device.KindVirtual}
	app := device.App{BundleID: "com.example.app", Type: "User"}
	result, cmd := m.Update(ui.RequestRebuildMsg{Device: dev, App: app})
	m2 := result.(model)
	if m2.installAppModal != nil {
		t.Error("expected no modal when project path already cached")
	}
	if cmd == nil {
		t.Fatal("expected rebuild cmd batch")
	}
	if !m2.installing {
		t.Error("expected installing flag set")
	}
}

func TestUpdate_ConfirmRebuildMsg_CachesAndroidPath(t *testing.T) {
	m := newTestModel()
	dev := device.Device{ID: "1", Platform: device.PlatformAndroid, Kind: device.KindVirtual}
	app := device.App{BundleID: "com.example.app", Type: "User"}
	result, cmd := m.Update(ui.ConfirmRebuildMsg{Device: dev, App: app, Path: "/some/project/dir"})
	m2 := result.(model)
	if m2.rebuildPaths["com.example.app"] != "/some/project/dir" {
		t.Errorf("expected path cached, got %q", m2.rebuildPaths["com.example.app"])
	}
	if m2.installAppModal != nil {
		t.Error("expected install modal cleared")
	}
	if cmd == nil {
		t.Error("expected rebuild cmd batch")
	}
}

func TestUpdate_ShowPlatformPickerMsg(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(ui.ShowPlatformPickerMsg{})
	m2 := result.(model)
	if m2.platformPicker == nil {
		t.Error("expected platformPicker set")
	}
}

func TestUpdate_ShowDeleteSimulatorMsg(t *testing.T) {
	m := newTestModel()
	dev := device.Device{Name: "iPhone"}
	result, _ := m.Update(ui.ShowDeleteSimulatorMsg{Device: dev})
	m2 := result.(model)
	if m2.deleteAlert == nil {
		t.Error("expected deleteAlert set")
	}
}

func TestUpdate_CreateSimulatorResult_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(createSimulatorResultMsg{err: errorString("create failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_CreateSimulatorResult_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(createSimulatorResultMsg{name: "NewSim"})
	m2 := result.(model)
	if !m2.loading {
		t.Error("expected loading=true")
	}
}

func TestUpdate_CreateAndroidEmulatorResult_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(createAndroidEmulatorResultMsg{err: errorString("create failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_DeleteSimulatorResult_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(deleteSimulatorResultMsg{err: errorString("delete failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_DeleteSimulatorResult_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(deleteSimulatorResultMsg{name: "OldSim"})
	m2 := result.(model)
	if !m2.loading {
		t.Error("expected loading=true")
	}
}

func TestUpdate_DeviceTypesMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(deviceTypesMsg{err: errorString("types failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_RuntimesMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runtimesMsg{err: errorString("runtimes failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_AndroidSystemImagesMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(androidSystemImagesMsg{err: errorString("images failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

func TestUpdate_AndroidDeviceProfilesMsg_Error(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(androidDeviceProfilesMsg{err: errorString("profiles failed")})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_DBViewerShowMsg(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(dbviewer.ShowMsg{})
	m2 := result.(model)
	if m2.dbViewerModal == nil {
		t.Error("expected dbViewerModal set")
	}
}

func TestUpdate_SQLiteVersionMsg_NoModal(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(dbviewer.SQLiteVersionMsg{})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_StopAppLoggingMsg_NoStream(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(ui.StopAppLoggingMsg{})
	m2 := result.(model)
	if m2.logBundleID != "" {
		t.Error("expected logBundleID cleared")
	}
}

func TestUpdate_BootPollMsg_DeviceRunning(t *testing.T) {
	m := newTestModel()
	dev := device.Device{ID: "abc", Status: device.StatusRunning}
	m.sidebar.SetDevices([]device.Device{dev})
	result, _ := m.Update(bootPollMsg{device: dev, remaining: 5})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_AutoRefreshMsg(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(autoRefreshMsg{})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_SQLiteResultMsg_NoModal(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(ui.SQLiteResultMsg{Rows: [][]string{{"a"}}})
	m2 := result.(model)
	_ = m2
}

func TestUpdate_ConfirmPlatformPickerMsg_IOS(t *testing.T) {
	m := newTestModel()
	p := ui.NewPlatformPickerModal()
	m.platformPicker = &p
	result, _ := m.Update(ui.ConfirmPlatformPickerMsg{Platform: device.PlatformIOS})
	m2 := result.(model)
	if m2.platformPicker != nil {
		t.Error("expected platformPicker nil after confirm")
	}
	if m2.createIOSModal == nil {
		t.Error("expected createIOSModal set")
	}
}

func TestUpdate_ConfirmPlatformPickerMsg_Android(t *testing.T) {
	m := newTestModel()
	p := ui.NewPlatformPickerModal()
	m.platformPicker = &p
	result, _ := m.Update(ui.ConfirmPlatformPickerMsg{Platform: device.PlatformAndroid})
	m2 := result.(model)
	if m2.createAndModal == nil {
		t.Error("expected createAndModal set")
	}
}

func TestUpdate_KeyEsc_FromMainFocus(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.applyFocus()
	// The main pane's esc chain unwinds one level per press (collapse the Apps
	// panel first); once there is nothing left it emits ui.ReleaseFocusMsg,
	// which the parent turns into a focus switch. Press until we get that cmd.
	var mm tea.Model = m
	var cmd tea.Cmd
	for i := 0; i < 4 && cmd == nil; i++ {
		mm, cmd = mm.(model).Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
	if cmd == nil {
		t.Fatal("expected a ReleaseFocusMsg command from esc in main focus")
	}
	mm, _ = mm.(model).Update(cmd())
	if mm.(model).focus != focusSidebar {
		t.Error("expected focus back to sidebar after esc → ReleaseFocusMsg")
	}
}

func TestUpdate_KeyR_RefreshLoading(t *testing.T) {
	m := newTestModel()
	m.loading = false
	result, cmd := m.Update(tea.KeyPressMsg{Code: 'r'})
	m1 := result.(model)
	if m1.loading {
		t.Error("loading should only flip once ui.RefreshDevicesMsg is processed")
	}
	if cmd == nil {
		t.Fatal("expected a cmd from r")
	}

	result, _ = m1.Update(cmd())
	m2 := result.(model)
	if !m2.loading {
		t.Error("expected loading=true after ui.RefreshDevicesMsg")
	}
}

func TestUpdate_AppsListMsg_Success(t *testing.T) {
	m := newTestModel()
	apps := []device.App{{BundleID: "com.test.app", Name: "Test"}}
	result, _ := m.Update(appsListMsg{apps: apps})
	m2 := result.(model)
	if len(m2.errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestUpdate_InfoMsg_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(infoMsg{info: device.DeviceInfo{}})
	m2 := result.(model)
	if len(m2.errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestUpdate_DeviceTypesMsg_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(deviceTypesMsg{types: []device.DeviceType{{Name: "iPhone 15"}}})
	m2 := result.(model)
	if len(m2.errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestUpdate_RuntimesMsg_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(runtimesMsg{runtimes: []device.Runtime{{Name: "iOS 17"}}})
	m2 := result.(model)
	if len(m2.errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestUpdate_AndroidSystemImagesMsg_Success(t *testing.T) {
	m := newTestModel()
	result, _ := m.Update(androidSystemImagesMsg{images: []device.Runtime{{Name: "android-33"}}})
	m2 := result.(model)
	if len(m2.errs) != 0 {
		t.Error("expected no errors")
	}
}

func TestUpdate_BootPollMsg_DeviceNotRunning(t *testing.T) {
	m := newTestModel()
	dev := device.Device{ID: "abc", Status: device.StatusOff}
	m.sidebar.SetDevices([]device.Device{dev})
	result, _ := m.Update(bootPollMsg{device: dev, remaining: 2})
	m2 := result.(model)
	_ = m2
}

func TestApplyFocus_SidebarFocus(t *testing.T) {
	m := newTestModel()
	m.focus = focusSidebar
	m.applyFocus()
}

func TestApplyFocus_MainFocus(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.applyFocus()
}

// ─── clearStatusMsg with installing/booting guards ────────────────────────

func TestUpdate_ClearStatusMsg_BlockedDuringInstall(t *testing.T) {
	m := newTestModel()
	m.status = "Building…"
	m.statusSeq = 1
	m.installing = true
	result, _ := m.Update(clearStatusMsg(1))
	m2 := result.(model)
	if m2.status == "" {
		t.Error("status must not clear while installing")
	}
}

func TestUpdate_ClearStatusMsg_BlockedDuringBoot(t *testing.T) {
	m := newTestModel()
	m.status = "Booting…"
	m.statusSeq = 1
	m.booting = true
	result, _ := m.Update(clearStatusMsg(1))
	m2 := result.(model)
	if m2.status == "" {
		t.Error("status must not clear while booting")
	}
}

// ─── logEndedMsg matching bundleID ────────────────────────────────────────

func TestUpdate_LogEndedMsg_MatchingBundleID_ClearsState(t *testing.T) {
	m := newTestModel()
	stopCalled := false
	m.logStream = &device.LogStream{
		Lines: make(chan string),
		Done:  make(chan error),
		Stop:  func() { stopCalled = true },
	}
	m.logBundleID = "com.example"
	m.logDeviceID = "device-1"
	dev := &device.Device{ID: "device-1", Status: device.StatusRunning}
	m.mainPane.SetDevice(dev, nil)
	m.mainPane.SetLoggingBundle("com.example")

	result, _ := m.Update(logEndedMsg{bundleID: "com.example"})
	m2 := result.(model)

	if m2.logStream != nil {
		t.Error("logStream must be nil after logEndedMsg")
	}
	if m2.logBundleID != "" {
		t.Errorf("logBundleID must be cleared, got %q", m2.logBundleID)
	}
	if m2.logDeviceID != "" {
		t.Errorf("logDeviceID must be cleared, got %q", m2.logDeviceID)
	}
	if m2.mainPane.LoggingBundle() != "" {
		t.Errorf("mainPane.LoggingBundle must be cleared, got %q", m2.mainPane.LoggingBundle())
	}
	_ = stopCalled // Stop not called by logEndedMsg; stream already ended
}

func TestUpdate_LogEndedMsg_WithError_AppendsError(t *testing.T) {
	m := newTestModel()
	m.logBundleID = "com.example"
	result, _ := m.Update(logEndedMsg{bundleID: "com.example", err: errorString("stream broke")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
}

// ─── discoveryMsg stops log stream when device gone ───────────────────────

func TestUpdate_DiscoveryMsg_StopsLogStreamWhenDeviceGone(t *testing.T) {
	m := newTestModel()
	stopCalled := false
	m.logStream = &device.LogStream{
		Lines: make(chan string),
		Done:  make(chan error),
		Stop:  func() { stopCalled = true },
	}
	m.logBundleID = "com.example"
	m.logDeviceID = "device-1"

	// discoveryMsg with no devices → mainPane has no device → stream must stop
	result, _ := m.Update(discoveryMsg{})
	m2 := result.(model)

	if !stopCalled {
		t.Error("Stop must be called when device disappears")
	}
	if m2.logStream != nil {
		t.Error("logStream must be nil after device disappears")
	}
	if m2.logBundleID != "" {
		t.Errorf("logBundleID must be cleared, got %q", m2.logBundleID)
	}
}

// ─── bootPollMsg clears booting at last poll ─────────────────────────────

func TestUpdate_BootPollMsg_LastPoll_ClearsBooting(t *testing.T) {
	m := newTestModel()
	m.booting = true
	dev := device.Device{ID: "abc", Status: device.StatusOff}
	m.sidebar.SetDevices([]device.Device{dev})
	result, _ := m.Update(bootPollMsg{device: dev, remaining: 1})
	m2 := result.(model)
	if m2.booting {
		t.Error("booting must be false after last poll")
	}
}

func TestUpdate_BootPollMsg_DeviceRunning_ClearsBooting(t *testing.T) {
	m := newTestModel()
	m.booting = true
	dev := device.Device{ID: "abc", Status: device.StatusRunning}
	m.sidebar.SetDevices([]device.Device{dev})
	result, _ := m.Update(bootPollMsg{device: dev, remaining: 5})
	m2 := result.(model)
	if m2.booting {
		t.Error("booting must be false when device confirmed running")
	}
}

// ─── bootResultMsg Android stays booting ─────────────────────────────────

func TestUpdate_BootResultMsg_Android_KeepsBooting(t *testing.T) {
	m := newTestModel()
	m.booting = true
	dev := device.Device{Name: "Pixel", Platform: device.PlatformAndroid}
	result, _ := m.Update(bootResultMsg{device: dev})
	m2 := result.(model)
	// Android boot starts poll; booting flag remains until poll resolves.
	if !m2.booting {
		t.Error("booting must stay true until Android poll confirms running")
	}
}

func TestUpdate_BootResultMsg_IOS_ClearsBooting(t *testing.T) {
	m := newTestModel()
	m.booting = true
	dev := device.Device{Name: "iPhone", Platform: device.PlatformIOS}
	result, _ := m.Update(bootResultMsg{device: dev})
	m2 := result.(model)
	if m2.booting {
		t.Error("booting must be false after iOS boot result")
	}
}

// ─── buildDoneMsg clears installing ──────────────────────────────────────

func TestUpdate_BuildDoneMsg_ClearsInstalling(t *testing.T) {
	m := newTestModel()
	m.installing = true
	result, _ := m.Update(buildDoneMsg{deviceID: "d1"})
	m2 := result.(model)
	if m2.installing {
		t.Error("installing must be false after buildDoneMsg")
	}
}

func TestUpdate_BuildDoneMsg_Error_AppendsError(t *testing.T) {
	m := newTestModel()
	m.installing = true
	result, _ := m.Update(buildDoneMsg{err: errorString("build failed")})
	m2 := result.(model)
	if len(m2.errs) == 0 {
		t.Error("expected error appended")
	}
	if m2.installing {
		t.Error("installing must be false even on error")
	}
}

// ─── StopAppLoggingMsg with active stream ─────────────────────────────────

func TestUpdate_StopAppLoggingMsg_WithActiveStream(t *testing.T) {
	m := newTestModel()
	stopCalled := false
	m.logStream = &device.LogStream{
		Lines: make(chan string),
		Done:  make(chan error),
		Stop:  func() { stopCalled = true },
	}
	m.logBundleID = "com.example"
	m.logDeviceID = "device-1"

	result, _ := m.Update(ui.StopAppLoggingMsg{})
	m2 := result.(model)

	if !stopCalled {
		t.Error("Stop must be called")
	}
	if m2.logStream != nil {
		t.Error("logStream must be nil")
	}
	if m2.logBundleID != "" || m2.logDeviceID != "" {
		t.Error("log bundle/device IDs must be cleared")
	}
}
