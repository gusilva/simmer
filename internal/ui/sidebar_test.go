package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
