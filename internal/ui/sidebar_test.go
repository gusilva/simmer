package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ─── SetFocused / SetSize / Devices ──────────────────────────────────────

func TestSidebar_SetFocused(t *testing.T) {
	s := NewSidebar()
	s.SetFocused(false)
	if s.outerFocused {
		t.Error("expected outerFocused false")
	}
	s.SetFocused(true)
	if !s.outerFocused {
		t.Error("expected outerFocused true")
	}
}

func TestSidebar_SetSize(t *testing.T) {
	s := NewSidebar()
	s.SetSize(50, 30)
	if s.Width() != 50 {
		t.Errorf("expected width 50, got %d", s.Width())
	}
	// Zero width must not overwrite.
	s.SetSize(0, 30)
	if s.Width() != 50 {
		t.Errorf("zero width must not change width; got %d", s.Width())
	}
}

func TestSidebar_Devices_ReturnsAll(t *testing.T) {
	s := NewSidebar()
	devs := []device.Device{
		{ID: "1", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "2", Status: device.StatusOff, Platform: device.PlatformIOS},
		{ID: "3", Status: device.StatusOff, Platform: device.PlatformAndroid},
	}
	s.SetDevices(devs)
	all := s.Devices()
	if len(all) != 3 {
		t.Errorf("expected 3 devices, got %d", len(all))
	}
}

func TestSidebar_SelectedDevice_BootedEmpty(t *testing.T) {
	s := NewSidebar()
	// No devices loaded — focused pane is PaneBooted, which is empty.
	sel := s.SelectedDevice()
	if sel != nil {
		t.Errorf("expected nil, got %v", sel)
	}
}

func TestSidebar_SelectedDevice_Available(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "avail", Name: "iPhone 14", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	// Switch to available pane and move past the iOS group header to the device row.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // skip header
	sel := s.SelectedDevice()
	if sel == nil || sel.ID != "avail" {
		t.Errorf("expected avail device selected, got %v", sel)
	}
}

func TestSidebar_View_Unfocused(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 30)
	s.SetFocused(false)
	view := s.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestSidebar_ToggleGroupCollapse(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone 14", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // go to available pane
	// availIdx=0 is the iOS header — enter toggles the current group.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !s.iosCollapsed {
		t.Error("expected iOS group collapsed after enter on header")
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.iosCollapsed {
		t.Error("expected iOS group expanded after second enter")
	}
}

func TestSidebar_FilterMode_ClearsOnEsc(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"}) // enter filter
	s, _ = s.Update(tea.KeyPressMsg{Text: "x"})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if s.filterMode || s.filterQuery != "" {
		t.Error("expected filter cleared after esc")
	}
}

func TestSidebar_BootedNavigation_UpDown(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "2", Name: "B", Status: device.StatusRunning, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if s.bootedIdx != 1 {
		t.Errorf("expected bootedIdx 1, got %d", s.bootedIdx)
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if s.bootedIdx != 0 {
		t.Errorf("expected bootedIdx 0, got %d", s.bootedIdx)
	}
}

func TestSidebar_View_WithFilter(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 30)
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone 14", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	s, _ = s.Update(tea.KeyPressMsg{Text: "i"})
	view := s.View()
	if view == "" {
		t.Error("expected non-empty view in filter mode")
	}
}

func TestSidebar(t *testing.T) {
	s := NewSidebar()

	// 1. Initial State
	if s.FocusedPane() != PaneBooted {
		t.Errorf("expected PaneBooted, got %v", s.FocusedPane())
	}

	// 2. SetDevices
	devs := []device.Device{
		{ID: "1", Name: "iPhone 15", Platform: device.PlatformIOS, Status: device.StatusRunning, Version: "17.0"},
		{ID: "2", Name: "iPhone 14", Platform: device.PlatformIOS, Status: device.StatusOff, Version: "16.0"},
		{ID: "3", Name: "Pixel 8", Platform: device.PlatformAndroid, Status: device.StatusOff, Version: "14"},
	}
	s.SetDevices(devs)

	if len(s.booted) != 1 {
		t.Errorf("expected 1 booted, got %d", len(s.booted))
	}
	if len(s.iosAvail) != 1 {
		t.Errorf("expected 1 ios avail, got %d", len(s.iosAvail))
	}
	if len(s.andAvail) != 1 {
		t.Errorf("expected 1 android avail, got %d", len(s.andAvail))
	}

	// 3. Navigation
	// Switch to Available
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if s.FocusedPane() != PaneAvailable {
		t.Errorf("expected PaneAvailable, got %v", s.FocusedPane())
	}

	// Move cursor down (should be on iOS header -> iPhone 14 -> Android header -> Pixel 8)
	// Initial availIdx is 0 (iOS header)
	if s.availIdx != 0 {
		t.Errorf("expected availIdx 0, got %d", s.availIdx)
	}

	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if s.availIdx != 1 {
		t.Errorf("expected availIdx 1 (iPhone 14), got %d", s.availIdx)
	}

	sel := s.SelectedDevice()
	if sel == nil || sel.Name != "iPhone 14" {
		t.Errorf("expected iPhone 14 selected, got %v", sel)
	}

	// 4. Filtering
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"}) // Enter filter mode
	if !s.filterMode {
		t.Error("expected filterMode to be true")
	}

	s, _ = s.Update(tea.KeyPressMsg{Text: "p"}) // Type 'p'
	s, _ = s.Update(tea.KeyPressMsg{Text: "i"}) // Type 'i'
	if s.filterQuery != "pi" {
		t.Errorf("expected filterQuery 'pi', got %q", s.filterQuery)
	}

	// Only Pixel 8 should match
	ios := s.filteredIOSAvail()
	and := s.filteredAndAvail()
	if len(ios) != 0 || len(and) != 1 {
		t.Errorf("filtering failed: ios=%d, and=%d", len(ios), len(and))
	}

	// Exit filter mode
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if s.filterMode {
		t.Error("expected filterMode to be false after esc")
	}

	// 5. View (smoke test)
	view := s.View()
	if !strings.Contains(view, "Booted") || !strings.Contains(view, "Available") {
		t.Error("View missing panel headers")
	}
	if lipgloss.Width(view) < s.Width() {
		t.Errorf("View width %d < sidebar width %d", lipgloss.Width(view), s.Width())
	}
}

func TestSidebar_Update_A_ShowsPlatformPicker(t *testing.T) {
	s := NewSidebar()
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // switch to available pane
	_, cmd := s.Update(tea.KeyPressMsg{Text: "a"})
	if cmd == nil {
		t.Fatal("expected cmd from 'a' in available pane")
	}
	if _, ok := cmd().(ShowPlatformPickerMsg); !ok {
		t.Error("expected ShowPlatformPickerMsg")
	}
}

func TestSidebar_Update_D_ShowsDeleteSimulator(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "avail1", Name: "iPhone 14", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // go to available pane
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // skip iOS header to device row
	_, cmd := s.Update(tea.KeyPressMsg{Text: "d"})
	if cmd == nil {
		t.Fatal("expected cmd from 'd' on device")
	}
	msg := cmd()
	del, ok := msg.(ShowDeleteSimulatorMsg)
	if !ok {
		t.Fatalf("expected ShowDeleteSimulatorMsg, got %T", msg)
	}
	if del.Device.ID != "avail1" {
		t.Errorf("wrong device: %v", del.Device)
	}
}

func TestSidebar_Update_D_NoDeviceAtPos_NoCmd(t *testing.T) {
	s := NewSidebar()
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// availIdx=0 is the iOS group header — no device here
	_, cmd := s.Update(tea.KeyPressMsg{Text: "d"})
	if cmd != nil {
		t.Error("expected no cmd when cursor is on group header")
	}
}

func TestSidebar_Update_Esc_ClearsFilterQuery(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Type "f" then "x" to set filterQuery without being in filterMode
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	s, _ = s.Update(tea.KeyPressMsg{Text: "x"}) // types in filterMode
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc}) // exits filterMode, keeps query
	// Now in nav mode with filterQuery="x" — esc should clear query
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if s.filterQuery != "" {
		t.Errorf("expected filterQuery cleared by esc in nav mode, got %q", s.filterQuery)
	}
}

func TestSidebar_FilterMode_Enter_ExitsFilter(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"}) // enter filter mode
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.filterMode {
		t.Error("expected filterMode=false after enter")
	}
}

func TestSidebar_FilterMode_BackspaceOnEmpty_NoOp(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"}) // enter filter mode
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyBackspace}) // nothing to delete
	if s.filterQuery != "" {
		t.Error("expected filterQuery still empty after backspace on empty")
	}
}
