package ui

import (
	"testing"

	"simmer/internal/device"
)

func TestSidebar_HandleClick_OnlineDeviceRow(t *testing.T) {
	s := NewSidebar()
	s.SetSize(36, 30)
	s.SetDevices([]device.Device{
		{ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})
	s.focused = PaneAvailable // prove the click also switches focus

	// positions: [header(iOS), device A] -> device row at y=2.
	s.HandleClick(2)
	if s.focused != PaneBooted {
		t.Errorf("expected focus PaneBooted, got %v", s.focused)
	}
	if got := s.SelectedDevice(); got == nil || got.ID != "a" {
		t.Errorf("expected selected device 'a', got %+v", got)
	}
}

func TestSidebar_HandleClick_OnlineHeaderRow(t *testing.T) {
	s := NewSidebar()
	s.SetSize(36, 30)
	s.SetDevices([]device.Device{
		{ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})

	s.HandleClick(1) // header row
	if s.focused != PaneBooted || s.bootedIdx != 0 {
		t.Errorf("expected focus PaneBooted, bootedIdx 0 (header), got focus=%v idx=%d", s.focused, s.bootedIdx)
	}
}

func TestSidebar_HandleClick_OfflineDeviceRow(t *testing.T) {
	s := NewSidebar()
	s.SetSize(36, 30)
	// No booted devices -> online box content is the single "(none)" row,
	// so onlineBoxH = 1 + 2 = 3.
	s.SetDevices([]device.Device{
		{ID: "x", Name: "X", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindVirtual},
		{ID: "y", Name: "Y", Status: device.StatusOff, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})

	// onlineBoxH=3, gap=1 -> offTop=4; localY=2 -> idx=1 (device Y) -> y=6.
	s.HandleClick(6)
	if s.focused != PaneAvailable {
		t.Errorf("expected focus PaneAvailable, got %v", s.focused)
	}
	if s.availIdx != 1 {
		t.Errorf("expected availIdx 1, got %d", s.availIdx)
	}
}

func TestSidebar_HandleClick_MixedPlatforms_GroupSeparatorRow(t *testing.T) {
	s := NewSidebar()
	s.SetSize(36, 30)
	s.SetDevices([]device.Device{
		{ID: "a", Name: "Alpha", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
		{ID: "b", Name: "Bravo", Status: device.StatusRunning, Platform: device.PlatformAndroid, Kind: device.KindVirtual},
		{ID: "c", Name: "Charlie", Status: device.StatusRunning, Platform: device.PlatformAndroid, Kind: device.KindVirtual},
	})

	// renderOnlineRows inserts a blank separator row between the iOS and
	// Android groups, so the rendered rows are:
	//   y=1 iOS header, y=2 Alpha, y=3 blank separator,
	//   y=4 Android header, y=5 Bravo, y=6 Charlie.
	s.HandleClick(5)
	if got := s.SelectedDevice(); got == nil || got.ID != "b" {
		t.Errorf("expected Bravo selected at y=5, got %+v", got)
	}

	s.HandleClick(6)
	if got := s.SelectedDevice(); got == nil || got.ID != "c" {
		t.Errorf("expected Charlie selected at y=6, got %+v", got)
	}

	s.HandleClick(3) // the blank separator row itself
	if got := s.SelectedDevice(); got == nil || got.ID != "c" {
		t.Errorf("expected separator-row click to leave selection unchanged (Charlie), got %+v", got)
	}
}

func TestSidebar_HandleClick_BorderRow_FocusOnlyNoCursorMove(t *testing.T) {
	s := NewSidebar()
	s.SetSize(36, 30)
	s.SetDevices([]device.Device{
		{ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS, Kind: device.KindVirtual},
	})
	s.focused = PaneAvailable
	s.bootedIdx = 1

	s.HandleClick(0) // top border row of the online box
	if s.focused != PaneBooted {
		t.Errorf("expected focus PaneBooted from border-row click, got %v", s.focused)
	}
	if s.bootedIdx != 1 {
		t.Errorf("border-row click must not move the cursor, got %d", s.bootedIdx)
	}
}
