package app

import (
	"testing"

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
