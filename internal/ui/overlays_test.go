package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

// ── CreateAndroidEmulatorModal ────────────────────────────────────────────

func TestCreateAndroidEmulatorModal_Loading(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	if !m.loading {
		t.Error("expected loading=true initially")
	}
	if !strings.Contains(m.View(), "Loading") {
		t.Error("expected loading message in view")
	}
}

func TestCreateAndroidEmulatorModal_SetSystemImages_AloneKeepsLoading(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	m.SetSystemImages([]device.Runtime{{Name: "API 34", Identifier: "s34", IsAvailable: true}})
	if !m.loading {
		t.Error("expected still loading when only system images set")
	}
}

func TestCreateAndroidEmulatorModal_SetBothLists_ClearsLoading(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	m.SetDeviceProfiles([]device.DeviceType{{Name: "Pixel 8", Identifier: "pixel_8"}})
	m.SetSystemImages([]device.Runtime{{Name: "API 34", Identifier: "s34", IsAvailable: true}})
	if m.loading {
		t.Error("expected loading=false after both lists set")
	}
}

func TestCreateAndroidEmulatorModal_View_Loaded(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	m.SetDeviceProfiles([]device.DeviceType{{Name: "Pixel 8", Identifier: "pixel_8"}})
	m.SetSystemImages([]device.Runtime{{Name: "API 34", Identifier: "s34", IsAvailable: true}})
	got := m.View()
	if !strings.Contains(got, "New Android Emulator") {
		t.Error("expected title in loaded view")
	}
	if !strings.Contains(got, "API 34") {
		t.Error("expected system image in view")
	}
}

func TestCreateAndroidEmulatorModal_Update_Esc_Cancels(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command from esc")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

func TestCreateAndroidEmulatorModal_Update_Tab_CyclesFocus(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	// Name → DeviceType
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focused != fieldDeviceType {
		t.Errorf("expected fieldDeviceType, got %v", m.focused)
	}
	// DeviceType → Runtime
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focused != fieldRuntime {
		t.Errorf("expected fieldRuntime, got %v", m.focused)
	}
	// Runtime → Name (wraps)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focused != fieldName {
		t.Errorf("expected fieldName after wrap, got %v", m.focused)
	}
}

func TestCreateAndroidEmulatorModal_Update_ShiftTab_CyclesBack(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	// Name → Runtime (wraps backward)
	m, _ = m.Update(tea.KeyPressMsg{Text: "shift+tab"})
	if m.focused != fieldRuntime {
		t.Errorf("expected fieldRuntime, got %v", m.focused)
	}
}

func TestCreateAndroidEmulatorModal_Update_Enter_SubmitWhenReady(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	m.SetDeviceProfiles([]device.DeviceType{{Name: "Pixel 8", Identifier: "pixel_8"}})
	m.SetSystemImages([]device.Runtime{{Name: "API 34", Identifier: "s34", IsAvailable: true}})
	// Set name
	m.nameInput.SetValue("MyEmulator")
	// Focus on Runtime (the "submit" field)
	m.focused = fieldRuntime
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd on submit")
	}
	msg := cmd()
	confirm, ok := msg.(ConfirmCreateAndroidEmulatorMsg)
	if !ok {
		t.Fatalf("expected ConfirmCreateAndroidEmulatorMsg, got %T", msg)
	}
	if confirm.Name != "MyEmulator" || confirm.SystemImagePkg != "s34" {
		t.Errorf("wrong confirm: %+v", confirm)
	}
}

func TestCreateAndroidEmulatorModal_Update_DownUp_Navigation(t *testing.T) {
	m, _ := NewCreateAndroidEmulatorModal()
	m.SetDeviceProfiles([]device.DeviceType{})
	m.SetSystemImages([]device.Runtime{
		{Name: "API 34", Identifier: "s34"},
		{Name: "API 35", Identifier: "s35"},
	})
	m.focused = fieldDeviceType
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.imgIdx != 1 {
		t.Errorf("expected imgIdx 1, got %d", m.imgIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.imgIdx != 0 {
		t.Errorf("expected imgIdx 0 after up, got %d", m.imgIdx)
	}
}

func TestPlatformPickerModal(t *testing.T) {
	m := NewPlatformPickerModal()

	// Initial selection (iOS)
	if m.idx != 0 {
		t.Errorf("expected idx 0, got %d", m.idx)
	}

	// Move down to Android
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.idx != 1 {
		t.Errorf("expected idx 1, got %d", m.idx)
	}

	// Confirm
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	confirm, ok := msg.(ConfirmPlatformPickerMsg)
	if !ok || confirm.Platform != device.PlatformAndroid {
		t.Errorf("expected ConfirmPlatformPickerMsg with Android, got %v", msg)
	}
}

func TestDeleteSimulatorAlert(t *testing.T) {
	dev := device.Device{Name: "iPhone 15", Platform: device.PlatformIOS}
	m := NewDeleteSimulatorAlert(dev)

	// Initial focus is Cancel
	if m.confirmFocus {
		t.Error("expected confirmFocus to be false (Cancel)")
	}

	// Toggle focus
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !m.confirmFocus {
		t.Error("expected confirmFocus to be true (Delete)")
	}

	// Confirm delete
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	confirm, ok := msg.(ConfirmDeleteSimulatorMsg)
	if !ok || confirm.Device.Name != "iPhone 15" {
		t.Errorf("expected ConfirmDeleteSimulatorMsg, got %v", msg)
	}
}

// ── DeleteSimulatorAlert.View ─────────────────────────────────────────────

func TestDeleteSimulatorAlert_View_iOS(t *testing.T) {
	dev := device.Device{Name: "iPhone 15", Platform: device.PlatformIOS}
	m := NewDeleteSimulatorAlert(dev)
	got := m.View()
	if !strings.Contains(got, "Delete Simulator") {
		t.Error("expected 'Delete Simulator' title for iOS")
	}
	if !strings.Contains(got, "iPhone 15") {
		t.Error("device name not in view")
	}
	if !strings.Contains(got, "cannot be undone") {
		t.Error("expected warning text in view")
	}
}

func TestDeleteSimulatorAlert_View_Android(t *testing.T) {
	dev := device.Device{Name: "Pixel 8", Platform: device.PlatformAndroid}
	m := NewDeleteSimulatorAlert(dev)
	got := m.View()
	if !strings.Contains(got, "Delete Emulator") {
		t.Error("expected 'Delete Emulator' title for Android")
	}
}

func TestDeleteSimulatorAlert_View_DeleteFocused(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	m := NewDeleteSimulatorAlert(dev)
	m.confirmFocus = true
	cancelUnfocused := m.View()
	m.confirmFocus = false
	cancelFocused := m.View()
	// The rendered buttons differ between focus states.
	if cancelUnfocused == cancelFocused {
		t.Error("focused and unfocused button views must differ")
	}
}

func TestDeleteSimulatorAlert_Update_Y_ConfirmDelete(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	m := NewDeleteSimulatorAlert(dev)
	_, cmd := m.Update(tea.KeyPressMsg{Text: "y"})
	if cmd == nil {
		t.Fatal("expected command from y key")
	}
	msg := cmd()
	if _, ok := msg.(ConfirmDeleteSimulatorMsg); !ok {
		t.Errorf("expected ConfirmDeleteSimulatorMsg, got %T", msg)
	}
}

func TestDeleteSimulatorAlert_Update_Esc_Cancels(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	m := NewDeleteSimulatorAlert(dev)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command from esc")
	}
	msg := cmd()
	if _, ok := msg.(CancelOverlayMsg); !ok {
		t.Errorf("expected CancelOverlayMsg, got %T", msg)
	}
}

func TestDeleteSimulatorAlert_Update_Enter_CancelFocused_Cancels(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	m := NewDeleteSimulatorAlert(dev)
	// confirmFocus=false → Enter = Cancel.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	if _, ok := msg.(CancelOverlayMsg); !ok {
		t.Errorf("expected CancelOverlayMsg, got %T", msg)
	}
}

// ── DeleteAppAlert ────────────────────────────────────────────────────────

func TestDeleteAppAlert_New(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.example.app", Name: "Example"}
	m := NewDeleteAppAlert(dev, app)
	if m.confirmFocus {
		t.Error("expected Cancel focused by default")
	}
}

func TestDeleteAppAlert_View(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.example.app", Name: "Example"}
	m := NewDeleteAppAlert(dev, app)
	got := m.View()
	if !strings.Contains(got, "Delete App") {
		t.Error("expected 'Delete App' title")
	}
	if !strings.Contains(got, "Example") {
		t.Error("expected app name in view")
	}
	if !strings.Contains(got, "com.example.app") {
		t.Error("expected bundle ID in view")
	}
}

func TestDeleteAppAlert_View_DeleteFocused(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.x", Name: "X"}
	m := NewDeleteAppAlert(dev, app)
	m.confirmFocus = true
	withDelete := m.View()
	m.confirmFocus = false
	withCancel := m.View()
	if withDelete == withCancel {
		t.Error("expected view to differ between focus states")
	}
}

func TestDeleteAppAlert_Update_Y_ConfirmDelete(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.x", Name: "X"}
	m := NewDeleteAppAlert(dev, app)
	_, cmd := m.Update(tea.KeyPressMsg{Text: "y"})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(ConfirmDeleteAppMsg); !ok {
		t.Errorf("expected ConfirmDeleteAppMsg, got %T", msg)
	}
}

func TestDeleteAppAlert_Update_Esc_Cancels(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.x"}
	m := NewDeleteAppAlert(dev, app)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd()
	if _, ok := msg.(CancelOverlayMsg); !ok {
		t.Errorf("expected CancelOverlayMsg, got %T", msg)
	}
}

func TestDeleteAppAlert_Update_Tab_TogglesConfirmFocus(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.x"}
	m := NewDeleteAppAlert(dev, app)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.confirmFocus {
		t.Error("expected confirmFocus toggled to true")
	}
}

func TestDeleteAppAlert_Update_Enter_DeleteFocused_Confirms(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.x"}
	m := NewDeleteAppAlert(dev, app)
	m.confirmFocus = true
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	if _, ok := msg.(ConfirmDeleteAppMsg); !ok {
		t.Errorf("expected ConfirmDeleteAppMsg, got %T", msg)
	}
}

func TestDeleteAppAlert_Update_NonKeyMsg_NoOp(t *testing.T) {
	dev := device.Device{Name: "iPhone"}
	app := device.App{BundleID: "com.x"}
	m := NewDeleteAppAlert(dev, app)
	m2, cmd := m.Update("not a key press")
	if cmd != nil {
		t.Error("expected no command for non-key message")
	}
	_ = m2
}

// ── HelpOverlay ───────────────────────────────────────────────────────────

func TestHelpOverlay_New(t *testing.T) {
	ov := NewHelpOverlay("Sidebar Help", SidebarKeys)
	got := ov.View()
	if !strings.Contains(got, "Sidebar Help") {
		t.Error("expected title in help overlay view")
	}
}

func TestHelpOverlay_View_ContainsDismissHint(t *testing.T) {
	ov := NewHelpOverlay("Main Pane", MainPaneKeys)
	got := ov.View()
	if !strings.Contains(got, "close") {
		t.Error("expected dismiss hint in help overlay")
	}
}

func TestHelpOverlay_View_ContainsKeys(t *testing.T) {
	ov := NewHelpOverlay("Global", GlobalKeys)
	got := ov.View()
	// GlobalKeys includes "?" and "q" bindings.
	if !strings.Contains(got, "?") {
		t.Error("expected ? binding in global help overlay")
	}
}

// ── PlatformPickerModal.View ──────────────────────────────────────────────

func TestPlatformPickerModal_View_IOSSelected(t *testing.T) {
	m := NewPlatformPickerModal()
	got := m.View()
	if !strings.Contains(got, "Add Device") {
		t.Error("expected 'Add Device' title in picker view")
	}
	if !strings.Contains(got, "iOS Simulator") {
		t.Error("expected 'iOS Simulator' option")
	}
	if !strings.Contains(got, "Android Emulator") {
		t.Error("expected 'Android Emulator' option")
	}
}

func TestPlatformPickerModal_View_AndroidSelected(t *testing.T) {
	m := NewPlatformPickerModal()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	got := m.View()
	// idx=1 = Android Emulator — shown with selection indicator ▸.
	if !strings.Contains(got, "▸") {
		t.Error("expected selection indicator ▸ in picker view")
	}
}

func TestPlatformPickerModal_Update_Esc_Cancels(t *testing.T) {
	m := NewPlatformPickerModal()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(CancelOverlayMsg); !ok {
		t.Errorf("expected CancelOverlayMsg, got %T", msg)
	}
}

func TestPlatformPickerModal_Update_Up_NoBounce(t *testing.T) {
	m := NewPlatformPickerModal()
	// Already at top — up should not go below 0.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.idx != 0 {
		t.Errorf("expected idx 0 at top, got %d", m.idx)
	}
}

func TestPlatformPickerModal_Update_Down_NoBounce(t *testing.T) {
	m := NewPlatformPickerModal()
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // already at bottom
	if m.idx != 1 {
		t.Errorf("expected idx 1 at bottom, got %d", m.idx)
	}
}

func TestCreateSimulatorModal_Esc_Cancels(t *testing.T) {
	m, _ := NewCreateSimulatorModal()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

func TestCreateSimulatorModal_Tab_CyclesFocus(t *testing.T) {
	m, _ := NewCreateSimulatorModal()
	m.SetDeviceTypes([]device.DeviceType{{Name: "iPhone 16", Identifier: "i16"}})
	m.SetRuntimes([]device.Runtime{{Name: "iOS 18", Identifier: "ios18", IsAvailable: true}})
	// Name → DeviceType
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focused != fieldDeviceType {
		t.Errorf("expected fieldDeviceType, got %v", m.focused)
	}
}

func TestCreateSimulatorModal_DownUp_OnDeviceType(t *testing.T) {
	m, _ := NewCreateSimulatorModal()
	m.SetDeviceTypes([]device.DeviceType{
		{Name: "iPhone 16", Identifier: "i16"},
		{Name: "iPhone 15", Identifier: "i15"},
	})
	m.SetRuntimes([]device.Runtime{{Name: "iOS 18", Identifier: "ios18", IsAvailable: true}})
	m.focused = fieldDeviceType
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.dtIdx != 1 {
		t.Errorf("expected dtIdx 1, got %d", m.dtIdx)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.dtIdx != 0 {
		t.Errorf("expected dtIdx 0 after up, got %d", m.dtIdx)
	}
}

func TestCreateSimulatorModal_View_Loaded(t *testing.T) {
	m, _ := NewCreateSimulatorModal()
	m.SetDeviceTypes([]device.DeviceType{{Name: "iPhone 16", Identifier: "i16"}})
	m.SetRuntimes([]device.Runtime{{Name: "iOS 18", Identifier: "ios18", IsAvailable: true}})
	got := m.View()
	if !strings.Contains(got, "iPhone 16") {
		t.Error("expected device type in view")
	}
	if !strings.Contains(got, "iOS 18") {
		t.Error("expected runtime in view")
	}
}

func TestCreateSimulatorModal_ShiftTab_CyclesBack(t *testing.T) {
	m, _ := NewCreateSimulatorModal()
	m.SetDeviceTypes([]device.DeviceType{{Name: "iPhone 16", Identifier: "i16"}})
	m.SetRuntimes([]device.Runtime{{Name: "iOS 18", Identifier: "ios18", IsAvailable: true}})
	// Name → Runtime (wraps backward)
	m, _ = m.Update(tea.KeyPressMsg{Text: "shift+tab"})
	if m.focused != fieldRuntime {
		t.Errorf("expected fieldRuntime, got %v", m.focused)
	}
}

func TestCreateSimulatorModal(t *testing.T) {
	m, _ := NewCreateSimulatorModal()

	// 1. Loading state
	if !m.loading {
		t.Error("expected loading to be true initially")
	}
	if !strings.Contains(m.View(), "Loading") {
		t.Error("View should show loading message")
	}

	// 2. Data population
	m.SetDeviceTypes([]device.DeviceType{{Name: "iPhone 16", Identifier: "i16"}})
	m.SetRuntimes([]device.Runtime{{Name: "iOS 18", Identifier: "ios18", IsAvailable: true}})

	if m.loading {
		t.Error("expected loading to be false after data population")
	}

	// 3. Input handling
	m, _ = m.Update(tea.KeyPressMsg{Text: "M"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "y"})
	if m.nameInput.Value() != "My" {
		t.Errorf("expected name 'My', got %q", m.nameInput.Value())
	}

	// 4. Submission
	// Focus is on Name. Enter moves to Device Type.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focused != fieldDeviceType {
		t.Errorf("expected focus on DeviceType, got %v", m.focused)
	}

	// Enter again moves to Runtime
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focused != fieldRuntime {
		t.Errorf("expected focus on Runtime, got %v", m.focused)
	}

	// Enter again submits if valid
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected submission command, got nil")
	}
	msg := cmd()
	confirm, ok := msg.(ConfirmCreateSimulatorMsg)
	if !ok || confirm.Name != "My" || confirm.DeviceTypeID != "i16" || confirm.RuntimeID != "ios18" {
		t.Errorf("invalid ConfirmCreateSimulatorMsg: %v", msg)
	}
}
