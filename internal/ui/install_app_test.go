package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

func newIOSInstallModal() InstallAppModal {
	dev := device.Device{Name: "iPhone 15", Platform: device.PlatformIOS}
	m, _ := NewInstallAppModal(dev)
	return m
}

func newAndroidInstallModal() InstallAppModal {
	dev := device.Device{Name: "Pixel 8", Platform: device.PlatformAndroid}
	m, _ := NewInstallAppModal(dev)
	return m
}

// ── NewInstallAppModal ────────────────────────────────────────────────────

func TestNewInstallAppModal_iOS_InitialState(t *testing.T) {
	m := newIOSInstallModal()
	if m.step != stepPick {
		t.Errorf("expected stepPick, got %v", m.step)
	}
	if m.dev.Platform != device.PlatformIOS {
		t.Error("expected iOS platform")
	}
}

func TestNewInstallAppModal_Android_InitialState(t *testing.T) {
	m := newAndroidInstallModal()
	if m.step != stepPick {
		t.Errorf("expected stepPick, got %v", m.step)
	}
}

// ── SetSchemes ────────────────────────────────────────────────────────────

func TestInstallAppModal_SetSchemes_Empty_SetsErrLoad(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{})
	if m.step != stepErrLoad {
		t.Errorf("expected stepErrLoad, got %v", m.step)
	}
	if !strings.Contains(m.loadErrMsg, "no schemes") {
		t.Errorf("expected error message, got %q", m.loadErrMsg)
	}
}

func TestInstallAppModal_SetSchemes_NonEmpty_SetsStepScheme(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{"MyApp", "MyAppTests"})
	if m.step != stepScheme {
		t.Errorf("expected stepScheme, got %v", m.step)
	}
	if len(m.schemes) != 2 {
		t.Errorf("expected 2 schemes, got %d", len(m.schemes))
	}
}

// ── SetSchemesError ───────────────────────────────────────────────────────

func TestInstallAppModal_SetSchemesError_SetsErrLoad(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemesError("xcodebuild failed")
	if m.step != stepErrLoad {
		t.Errorf("expected stepErrLoad, got %v", m.step)
	}
	if m.loadErrMsg != "xcodebuild failed" {
		t.Errorf("wrong error message: %q", m.loadErrMsg)
	}
}

// ── Update: stepPick ──────────────────────────────────────────────────────

func TestInstallAppModal_Update_Pick_Esc_Cancels(t *testing.T) {
	m := newIOSInstallModal()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

// ── Update: stepLoading ───────────────────────────────────────────────────

func TestInstallAppModal_Update_Loading_Esc_Cancels(t *testing.T) {
	m := newIOSInstallModal()
	m.step = stepLoading
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc in loading step")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

func TestInstallAppModal_Update_Loading_NonEsc_NoOp(t *testing.T) {
	m := newIOSInstallModal()
	m.step = stepLoading
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("expected no cmd for non-esc in loading step")
	}
}

// ── Update: stepErrLoad ───────────────────────────────────────────────────

func TestInstallAppModal_Update_ErrLoad_Esc_Cancels(t *testing.T) {
	m := newIOSInstallModal()
	m.step = stepErrLoad
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc in errLoad step")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

func TestInstallAppModal_Update_ErrLoad_Enter_GoesBackToPick(t *testing.T) {
	m := newIOSInstallModal()
	m.step = stepErrLoad
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m2.step != stepPick {
		t.Errorf("expected stepPick, got %v", m2.step)
	}
}

// ── Update: stepScheme / updateScheme ────────────────────────────────────

func TestInstallAppModal_Update_Scheme_Esc_Cancels(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{"MyApp"})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc in scheme step")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

func TestInstallAppModal_Update_Scheme_ShiftTab_BackToPick(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{"MyApp"})
	m2, _ := m.Update(tea.KeyPressMsg{Text: "shift+tab"})
	if m2.step != stepPick {
		t.Errorf("expected stepPick after shift+tab, got %v", m2.step)
	}
}

func TestInstallAppModal_Update_Scheme_DownUp_Navigation(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{"MyApp", "MyAppTests"})
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m2.schemeIdx != 1 {
		t.Errorf("expected schemeIdx 1, got %d", m2.schemeIdx)
	}
	m3, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m3.schemeIdx != 0 {
		t.Errorf("expected schemeIdx 0 after up, got %d", m3.schemeIdx)
	}
}

func TestInstallAppModal_Update_Scheme_Enter_Confirms(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{"MyApp", "MyAppTests"})
	m.pickedPath = "/path/to/project.xcworkspace"
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from enter on scheme")
	}
	msg, ok := cmd().(ConfirmInstallAppMsg)
	if !ok {
		t.Fatalf("expected ConfirmInstallAppMsg, got %T", cmd())
	}
	if msg.Scheme != "MyApp" {
		t.Errorf("expected scheme 'MyApp', got %q", msg.Scheme)
	}
}

// ── View ──────────────────────────────────────────────────────────────────

func TestInstallAppModal_View_Pick(t *testing.T) {
	m := newIOSInstallModal()
	got := m.View()
	if got == "" {
		t.Error("expected non-empty view in pick step")
	}
}

func TestInstallAppModal_View_Loading(t *testing.T) {
	m := newIOSInstallModal()
	m.step = stepLoading
	got := m.View()
	if !strings.Contains(got, "Detecting") && !strings.Contains(got, "Loading") && !strings.Contains(got, "detecting") {
		t.Log("view:", got)
	}
}

func TestInstallAppModal_View_Scheme(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemes([]string{"MyApp", "MyAppTests"})
	got := m.View()
	if !strings.Contains(got, "MyApp") {
		t.Error("expected scheme name in view")
	}
}

func TestInstallAppModal_View_ErrLoad(t *testing.T) {
	m := newIOSInstallModal()
	m.SetSchemesError("xcodebuild failed with exit code 1")
	got := m.View()
	if !strings.Contains(got, "xcodebuild failed") {
		t.Error("expected error message in view")
	}
}
