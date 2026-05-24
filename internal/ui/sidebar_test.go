package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ─── SetFocused / SetSize / Width ────────────────────────────────────────────

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
	s.SetSize(0, 30)
	if s.Width() != 50 {
		t.Errorf("zero width must not change width; got %d", s.Width())
	}
}

// ─── SetDevices routing ──────────────────────────────────────────────────────

func TestSetDevices_Routing(t *testing.T) {
	s := NewSidebar()
	devs := []device.Device{
		{ID: "r1", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
		{ID: "r2", Status: device.StatusRunning, Platform: device.PlatformAndroid, Kind: device.KindPhysical},
		{ID: "vios", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindVirtual},
		{ID: "vand", Status: device.StatusOff, Platform: device.PlatformAndroid, Kind: device.KindVirtual},
		{ID: "skip", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindPhysical},
	}
	s.SetDevices(devs)

	if len(s.booted) != 2 {
		t.Errorf("booted: want 2, got %d", len(s.booted))
	}
	if len(s.virtIOS) != 1 || s.virtIOS[0].ID != "vios" {
		t.Errorf("virtIOS: want [vios], got %v", s.virtIOS)
	}
	if len(s.virtAnd) != 1 || s.virtAnd[0].ID != "vand" {
		t.Errorf("virtAnd: want [vand], got %v", s.virtAnd)
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

// ─── SelectedDevice ──────────────────────────────────────────────────────────

func TestSidebar_SelectedDevice_OnlineEmpty(t *testing.T) {
	s := NewSidebar()
	if sel := s.SelectedDevice(); sel != nil {
		t.Errorf("expected nil, got %v", sel)
	}
}

func TestSidebar_SelectedDevice_Online(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "r", Name: "iPhone 15", Status: device.StatusRunning, Platform: device.PlatformIOS},
	})
	// bootedIdx=0 is the iOS header — SelectedDevice returns nil for headers.
	if sel := s.SelectedDevice(); sel != nil {
		t.Errorf("cursor on header: expected nil, got %v", sel)
	}
	// Move down to the device row.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	sel := s.SelectedDevice()
	if sel == nil || sel.ID != "r" {
		t.Errorf("expected device r, got %v", sel)
	}
}

func TestSidebar_SelectedDevice_Offline(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "avail", Name: "iPhone 14", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	// Switch to offline pane; flat list, device at index 0.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	sel := s.SelectedDevice()
	if sel == nil || sel.ID != "avail" {
		t.Errorf("expected avail device, got %v", sel)
	}
}

// ─── Online box group navigation and collapse ────────────────────────────────

func TestOnlineBox_GroupNavigation(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "ios1", Name: "iPhone 15", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "and1", Name: "Pixel 9", Status: device.StatusRunning, Platform: device.PlatformAndroid},
	})
	// Initial: bootedIdx=0 → iOS header.
	pos := s.onlinePositions()
	if len(pos) < 4 {
		t.Fatalf("expected ≥4 positions (iOS hdr, ios1, And hdr, and1), got %d", len(pos))
	}
	if !pos[0].isHeader || pos[0].platform != device.PlatformIOS {
		t.Error("pos[0] should be iOS header")
	}
	if pos[1].isHeader {
		t.Error("pos[1] should be device, not header")
	}
	if !pos[2].isHeader || pos[2].platform != device.PlatformAndroid {
		t.Error("pos[2] should be Android header")
	}
}

func TestOnlineBox_CollapseExpand(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "ios1", Name: "iPhone 15", Status: device.StatusRunning, Platform: device.PlatformIOS},
	})
	// bootedIdx=0 is the iOS header; Enter toggles collapse.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !s.iosCollapsed {
		t.Error("expected iOS group collapsed after Enter on header")
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.iosCollapsed {
		t.Error("expected iOS group expanded after second Enter")
	}
}

func TestOnlineBox_CollapseExpand_CursorReanchors(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "ios1", Status: device.StatusRunning, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // collapse
	// After collapse, cursor should still be on the iOS header (bootedIdx=0).
	if s.bootedIdx != 0 {
		t.Errorf("cursor should re-anchor to header; got bootedIdx=%d", s.bootedIdx)
	}
}

func TestOnlineBox_Pagination(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "i1", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "a1", Status: device.StatusRunning, Platform: device.PlatformAndroid},
	})
	// bootedIdx=0 → iOS header → x=1 of 2
	p := s.onlinePaginationStr()
	if p != "1 of 2" {
		t.Errorf("on iOS header: want '1 of 2', got %q", p)
	}
	// Move to iOS device (idx 1) → x=1 of 2
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p = s.onlinePaginationStr()
	if p != "1 of 2" {
		t.Errorf("on iOS device: want '1 of 2', got %q", p)
	}
	// Move to Android header (idx 2) → x=2 of 2
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	p = s.onlinePaginationStr()
	if p != "2 of 2" {
		t.Errorf("on Android header: want '2 of 2', got %q", p)
	}
}

func TestOnlineBox_MixedKinds(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "vios", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
		{ID: "pios", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindPhysical},
	})
	// Both appear under the same iOS header.
	if len(s.booted) != 2 {
		t.Errorf("both running devices should be in booted; got %d", len(s.booted))
	}
	pos := s.onlinePositions()
	// Should be: iOS_hdr, vios_dev, pios_dev
	if len(pos) != 3 {
		t.Errorf("expected 3 positions; got %d", len(pos))
	}
}

// ─── Offline box tab switch ──────────────────────────────────────────────────

func TestOfflineBox_TabSwitch(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "ios1", Status: device.StatusOff, Platform: device.PlatformIOS},
		{ID: "and1", Status: device.StatusOff, Platform: device.PlatformAndroid},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // go to offline pane
	// Default tab is iOS.
	if s.offlinePlatform != tabIOS {
		t.Error("default offline tab should be iOS")
	}
	devs := s.offlineDevices()
	if len(devs) != 1 || devs[0].ID != "ios1" {
		t.Errorf("iOS tab: expected [ios1], got %v", devs)
	}

	// Switch to Android with ].
	s, _ = s.Update(tea.KeyPressMsg{Text: "]"})
	if s.offlinePlatform != tabAndroid {
		t.Error("expected Android tab after ]")
	}
	if s.availIdx != 0 {
		t.Errorf("tab switch must reset availIdx; got %d", s.availIdx)
	}
	devs = s.offlineDevices()
	if len(devs) != 1 || devs[0].ID != "and1" {
		t.Errorf("Android tab: expected [and1], got %v", devs)
	}

	// Switch back to iOS with [.
	s, _ = s.Update(tea.KeyPressMsg{Text: "["})
	if s.offlinePlatform != tabIOS {
		t.Error("expected iOS tab after [")
	}
}

func TestOfflineBox_FilterPreservedOnSwitch(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "i1", Name: "iPhone 14", Status: device.StatusOff, Platform: device.PlatformIOS},
		{ID: "i2", Name: "iPad Pro", Status: device.StatusOff, Platform: device.PlatformIOS},
		{ID: "a1", Name: "Pixel 9", Status: device.StatusOff, Platform: device.PlatformAndroid},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // go to offline
	// Enter filter mode and type "ip" (matches iPad).
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	s, _ = s.Update(tea.KeyPressMsg{Text: "i"})
	s, _ = s.Update(tea.KeyPressMsg{Text: "p"})
	if s.filterQuery != "ip" {
		t.Fatalf("expected filterQuery 'ip', got %q", s.filterQuery)
	}

	// Switch to Android: query preserved, re-evaluated for Android list.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // exit filter mode, keep query
	s, _ = s.Update(tea.KeyPressMsg{Text: "]"})
	if s.filterQuery != "ip" {
		t.Errorf("filter query should be preserved after tab switch; got %q", s.filterQuery)
	}
	// Android tab with query "ip" — no match → empty.
	andDevs := s.offlineDevices()
	if len(andDevs) != 0 {
		t.Errorf("expected no Android match for 'ip'; got %v", andDevs)
	}
}

func TestOfflineBox_PhysicalDeviceExcluded(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "phys", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindPhysical},
		{ID: "virt", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})
	if len(s.virtIOS) != 1 || s.virtIOS[0].ID != "virt" {
		t.Errorf("only virtual iOS should be in virtIOS; got %v", s.virtIOS)
	}
}

// ─── View width and empty state ──────────────────────────────────────────────

func TestView_Width(t *testing.T) {
	devs := []device.Device{
		{ID: "r", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "v", Status: device.StatusOff, Platform: device.PlatformIOS},
	}
	for _, w := range []int{30, 36, 50} {
		s := NewSidebar()
		s.SetSize(w, 40)
		s.SetDevices(devs)
		view := s.View()
		for i, line := range strings.Split(view, "\n") {
			if lw := lipgloss.Width(line); lw != w {
				t.Errorf("width=%d: line %d width = %d", w, i, lw)
			}
		}
	}
}

func TestView_EmptyState(t *testing.T) {
	s := NewSidebar()
	s.SetSize(36, 40)
	view := s.View()
	if view == "" {
		t.Error("expected non-empty view with no devices")
	}
	if !strings.Contains(view, "(none)") {
		t.Error("expected '(none)' in empty view")
	}
}

func TestSidebar_View_Unfocused(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 30)
	s.SetFocused(false)
	if s.View() == "" {
		t.Error("expected non-empty view when unfocused")
	}
}

// ─── Filter mode ─────────────────────────────────────────────────────────────

func TestSidebar_FilterMode_ClearsOnEsc(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	s, _ = s.Update(tea.KeyPressMsg{Text: "x"})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc}) // exits filterMode, keeps query
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc}) // clears query in nav mode
	if s.filterMode || s.filterQuery != "" {
		t.Error("expected filter cleared after esc in nav mode")
	}
}

func TestSidebar_FilterMode_Enter_ExitsFilter(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if s.filterMode {
		t.Error("expected filterMode=false after Enter")
	}
}

func TestSidebar_FilterMode_BackspaceOnEmpty_NoOp(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "iPhone", Status: device.StatusOff, Platform: device.PlatformIOS},
	})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if s.filterQuery != "" {
		t.Error("filterQuery should still be empty after backspace on empty")
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
	if s.View() == "" {
		t.Error("expected non-empty view in filter mode")
	}
}

// ─── Online navigation ───────────────────────────────────────────────────────

func TestSidebar_OnlineNavigation_UpDown(t *testing.T) {
	s := NewSidebar()
	s.SetDevices([]device.Device{
		{ID: "1", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "2", Name: "B", Status: device.StatusRunning, Platform: device.PlatformIOS},
	})
	// Positions: iOS_hdr(0), dev1(1), dev2(2)
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // 0→1
	if s.bootedIdx != 1 {
		t.Errorf("expected bootedIdx 1, got %d", s.bootedIdx)
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyUp}) // 1→0
	if s.bootedIdx != 0 {
		t.Errorf("expected bootedIdx 0, got %d", s.bootedIdx)
	}
}

// ─── Key: 1 and 2 focus panes ────────────────────────────────────────────────

func TestSidebar_Key1And2_FocusPanes(t *testing.T) {
	s := NewSidebar()
	s, _ = s.Update(tea.KeyPressMsg{Text: "2"})
	if s.focused != PaneAvailable {
		t.Errorf("'2' should focus PaneAvailable; got %v", s.focused)
	}
	s, _ = s.Update(tea.KeyPressMsg{Text: "1"})
	if s.focused != PaneBooted {
		t.Errorf("'1' should focus PaneBooted; got %v", s.focused)
	}
}

// ─── Key: a and d ────────────────────────────────────────────────────────────

func TestSidebar_Update_A_ShowsPlatformPicker(t *testing.T) {
	s := NewSidebar()
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	_, cmd := s.Update(tea.KeyPressMsg{Text: "a"})
	if cmd == nil {
		t.Fatal("expected cmd from 'a' in offline pane")
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
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // go to offline pane
	// availIdx=0 is the device (flat list, no headers).
	_, cmd := s.Update(tea.KeyPressMsg{Text: "d"})
	if cmd == nil {
		t.Fatal("expected cmd from 'd' on device")
	}
	del, ok := cmd().(ShowDeleteSimulatorMsg)
	if !ok {
		t.Fatalf("expected ShowDeleteSimulatorMsg, got %T", cmd())
	}
	if del.Device.ID != "avail1" {
		t.Errorf("wrong device: %v", del.Device)
	}
}

func TestSidebar_Update_D_NoDeviceAtPos_NoCmd(t *testing.T) {
	s := NewSidebar()
	// No offline devices → d should emit nothing.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	_, cmd := s.Update(tea.KeyPressMsg{Text: "d"})
	if cmd != nil {
		t.Error("expected no cmd when offline list is empty")
	}
}

// ─── Comprehensive integration test ─────────────────────────────────────────

func TestSidebar(t *testing.T) {
	s := NewSidebar()

	if s.FocusedPane() != PaneBooted {
		t.Errorf("expected PaneBooted, got %v", s.FocusedPane())
	}

	devs := []device.Device{
		{ID: "1", Name: "iPhone 15", Platform: device.PlatformIOS, Status: device.StatusRunning, Version: "17.0"},
		{ID: "2", Name: "iPhone 14", Platform: device.PlatformIOS, Status: device.StatusOff, Version: "16.0"},
		{ID: "3", Name: "Pixel 8", Platform: device.PlatformAndroid, Status: device.StatusOff, Version: "14"},
	}
	s.SetDevices(devs)

	if len(s.booted) != 1 {
		t.Errorf("expected 1 booted, got %d", len(s.booted))
	}
	if len(s.virtIOS) != 1 {
		t.Errorf("expected 1 virtIOS, got %d", len(s.virtIOS))
	}
	if len(s.virtAnd) != 1 {
		t.Errorf("expected 1 virtAnd, got %d", len(s.virtAnd))
	}

	// Switch to offline pane.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if s.FocusedPane() != PaneAvailable {
		t.Errorf("expected PaneAvailable, got %v", s.FocusedPane())
	}

	// Offline pane is a flat list; availIdx=0 → first device.
	if s.availIdx != 0 {
		t.Errorf("expected availIdx 0, got %d", s.availIdx)
	}
	sel := s.SelectedDevice()
	if sel == nil || sel.Name != "iPhone 14" {
		t.Errorf("expected iPhone 14 selected, got %v", sel)
	}

	// Filtering.
	s, _ = s.Update(tea.KeyPressMsg{Text: "f"})
	if !s.filterMode {
		t.Error("expected filterMode true")
	}
	s, _ = s.Update(tea.KeyPressMsg{Text: "p"})
	s, _ = s.Update(tea.KeyPressMsg{Text: "i"})
	if s.filterQuery != "pi" {
		t.Errorf("expected filterQuery 'pi', got %q", s.filterQuery)
	}

	// On iOS tab: "pi" matches nothing.
	iosFiltered := s.filteredVirtIOS()
	andFiltered := s.filteredVirtAnd()
	if len(iosFiltered) != 0 {
		t.Errorf("expected 0 iOS matches for 'pi', got %d", len(iosFiltered))
	}
	// "Pixel 8" matches "pi" on Android tab.
	if len(andFiltered) != 1 {
		t.Errorf("expected 1 Android match for 'pi', got %d", len(andFiltered))
	}

	// Exit filter mode.
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if s.filterMode {
		t.Error("expected filterMode false after esc")
	}

	// View smoke test: both box labels present.
	s.SetSize(36, 40)
	view := s.View()
	if !strings.Contains(view, "Online") {
		t.Error("View missing 'Online' box label")
	}
	if !strings.Contains(view, "iOS") {
		t.Error("View missing 'iOS' tab in offline box")
	}
	if lipgloss.Width(view) < s.Width() {
		t.Errorf("View width %d < sidebar width %d", lipgloss.Width(view), s.Width())
	}
}
