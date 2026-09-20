package app

import (
	"strings"
	"testing"
	"time"

	"simmer/internal/device"
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
)

func TestMouseClick_Sidebar_SetsFocusAndClamps(t *testing.T) {
	m := newTestModel()
	m.focus = focusMain
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}

	result, _ := m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	m2 := result.(model)
	if m2.focus != focusSidebar {
		t.Errorf("expected focus switched to sidebar, got %v", m2.focus)
	}
}

func TestMouseClick_MainPane_SetsFocus(t *testing.T) {
	m := newTestModel()
	m.focus = focusSidebar
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}

	result, _ := m.Update(tea.MouseClickMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	m2 := result.(model)
	if m2.focus != focusMain {
		t.Errorf("expected focus switched to main, got %v", m2.focus)
	}
}

func TestMouseClick_OutsideAnyRegion_NoOp(t *testing.T) {
	m := newTestModel()
	m.focus = focusSidebar
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}

	result, _ := m.Update(tea.MouseClickMsg{X: 200, Y: 200, Button: tea.MouseLeft})
	m2 := result.(model)
	if m2.focus != focusSidebar {
		t.Errorf("expected focus unchanged, got %v", m2.focus)
	}
}

func TestMouseClick_RightButton_Ignored(t *testing.T) {
	m := newTestModel()
	m.focus = focusSidebar
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}

	result, _ := m.Update(tea.MouseClickMsg{X: 40, Y: 5, Button: tea.MouseRight})
	m2 := result.(model)
	if m2.focus != focusSidebar {
		t.Errorf("expected right-click to be ignored, focus stayed sidebar, got %v", m2.focus)
	}
}

func TestMouseClick_ComplexModalOpen_SwallowsClick(t *testing.T) {
	m := newTestModel()
	m.focus = focusSidebar
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}
	picker := ui.NewPlatformPickerModal()
	m.platformPicker = &picker

	result, _ := m.Update(tea.MouseClickMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	m2 := result.(model)
	if m2.focus != focusSidebar {
		t.Errorf("expected click swallowed by open modal, focus unchanged, got %v", m2.focus)
	}
}

func TestMouseClick_SimpleAlert_OutsideDismisses(t *testing.T) {
	m := newTestModel()
	alert := ui.NewDeleteSimulatorAlert(device.Device{ID: "1", Name: "A"})
	m.deleteAlert = &alert
	m.layout.overlay = rect{x: 10, y: 10, w: 20, h: 5}

	result, cmd := m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	m2 := result.(model)
	if cmd == nil {
		t.Fatal("expected a cancel cmd from outside-click dismiss")
	}
	if msg := cmd(); msg != (ui.CancelOverlayMsg{}) {
		t.Errorf("expected ui.CancelOverlayMsg, got %#v", msg)
	}
	_ = m2
}

func TestMouseClick_SimpleAlert_InsideSwallowed(t *testing.T) {
	m := newTestModel()
	alert := ui.NewDeleteSimulatorAlert(device.Device{ID: "1", Name: "A"})
	m.deleteAlert = &alert
	m.layout.overlay = rect{x: 10, y: 10, w: 20, h: 5}

	result, cmd := m.Update(tea.MouseClickMsg{X: 12, Y: 12, Button: tea.MouseLeft})
	m2 := result.(model)
	if cmd != nil {
		t.Errorf("expected inside click to be swallowed with no cmd, got one")
	}
	if m2.deleteAlert == nil {
		t.Error("expected alert to remain open")
	}
}

// ── double-click action menu ────────────────────────────────────────────

func TestRegisterClick_RequiresPreSelectionOnBothClicksAndWithinThreshold(t *testing.T) {
	m := newTestModel()

	// Click 1: row "a" wasn't selected before this click (preID differs) ->
	// not a double, and doesn't count as "pre-selected" for a future pair.
	if m.registerClick("a", "") {
		t.Fatal("first click on a newly-selected row must not be a double-click")
	}
	// Click 2: row "a" is now selected and this click reselects it, but the
	// row wasn't selected *before the pair started* (click 1 is what
	// selected it) -> still not a double.
	if m.registerClick("a", "a") {
		t.Fatal("click pair starting from an unselected row must not open the menu")
	}
	// Click 3: row "a" was already selected before this click (by click 2),
	// and click 2 was itself a reselect -> this completes a valid double.
	if !m.registerClick("a", "a") {
		t.Fatal("expected double-click once two consecutive reselect-clicks land on the same row")
	}
}

func TestRegisterClick_DifferentRowResetsSequence(t *testing.T) {
	m := newTestModel()
	m.registerClick("a", "a")
	if m.registerClick("b", "a") {
		t.Fatal("clicking a different row must not trigger a double-click")
	}
}

func TestRegisterClick_ExpiredThresholdDoesNotFire(t *testing.T) {
	m := newTestModel()
	m.registerClick("a", "a")
	m.lastClick.at = time.Now().Add(-2 * doubleClickThreshold)
	if m.registerClick("a", "a") {
		t.Fatal("expected stale click pair to miss the double-click threshold")
	}
}

func TestBuildDeviceActionMenu_Running_IncludesLoadAndRefresh(t *testing.T) {
	m := newTestModel()
	menu := m.buildDeviceActionMenu(device.Device{
		ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual,
	})
	view := menu.View()
	for _, label := range []string{"Load Device", "Shutdown", "Add Device", "Refresh"} {
		if !strings.Contains(view, label) {
			t.Errorf("expected running-device menu to include %q, got:\n%s", label, view)
		}
	}
}

func TestBuildDeviceActionMenu_Offline_IncludesBootDeleteAddRefresh(t *testing.T) {
	m := newTestModel()
	menu := m.buildDeviceActionMenu(device.Device{
		ID: "a", Name: "A", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindVirtual,
	})
	view := menu.View()
	for _, label := range []string{"Boot", "Delete", "Add Device", "Refresh"} {
		if !strings.Contains(view, label) {
			t.Errorf("expected offline-device menu to include %q, got:\n%s", label, view)
		}
	}
}

func TestMouseClick_Sidebar_DoubleClickOnPreSelectedRow_OpensActionMenu(t *testing.T) {
	m := newTestModel()
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}
	m.sidebar.SetDevices([]device.Device{
		{ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})
	// Cursor starts on the group header (not the device row): an initial
	// click is needed to select "a" before the double-click pair begins.
	result, _ := m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	m0 := result.(model)
	if got := m0.sidebar.SelectedDevice(); got == nil || got.ID != "a" {
		t.Fatalf("expected device a selected after the priming click, got %+v", got)
	}

	result, _ = m0.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	m1 := result.(model)
	if m1.actionMenu != nil {
		t.Fatal("first click of the double-click pair must not open the action menu")
	}

	result, _ = m1.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	m2 := result.(model)
	if m2.actionMenu == nil {
		t.Fatal("expected the double-click pair on the already-selected row to open the action menu")
	}
}

func TestMouseClick_Sidebar_DoubleClickAfterSelectionChange_DoesNotOpenMenu(t *testing.T) {
	m := newTestModel()
	m.layout.sidebar = rect{x: 0, y: 2, w: 38, h: 20}
	m.layout.mainPane = rect{x: 38, y: 2, w: 40, h: 20}
	m.sidebar.SetDevices([]device.Device{
		{ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
		{ID: "b", Name: "B", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})
	// Prime by selecting "a" (row y=5), then click "b" (row y=6): that click
	// itself changes the selection, so it can't start a double-click pair.
	result, _ := m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	m0 := result.(model)

	result, _ = m0.Update(tea.MouseClickMsg{X: 5, Y: 6, Button: tea.MouseLeft})
	m1 := result.(model)
	if got := m1.sidebar.SelectedDevice(); got == nil || got.ID != "b" {
		t.Fatalf("expected selection moved to b, got %+v", got)
	}

	result, _ = m1.Update(tea.MouseClickMsg{X: 5, Y: 6, Button: tea.MouseLeft})
	m2 := result.(model)
	if m2.actionMenu != nil {
		t.Fatal("row wasn't selected before this click pair started; menu must not open")
	}
}
