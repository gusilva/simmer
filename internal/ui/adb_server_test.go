package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderTopBar_AdbServerBadge(t *testing.T) {
	tests := []struct {
		state   device.AdbServerState
		version string
		want    string
		notWant string
	}{
		{device.AdbServerRunning, "1.0.41", "adb 1.0.41 ●", "server down"},
		{device.AdbServerStopped, "1.0.41", "adb 1.0.41 ● server down", ""},
		{device.AdbNotInstalled, "n/a", "adb n/a", "●  "},
		{device.AdbServerUnknown, "1.0.41", "adb 1.0.41", "server down"},
	}
	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			got := RenderTopBar(TopBarParams{
				Width:        120,
				ToolVersions: map[device.Platform]string{device.PlatformAndroid: tt.version},
				AdbServer:    tt.state,
			})
			plain := ansi.Strip(got)
			if !strings.Contains(plain, tt.want) {
				t.Errorf("top bar %q missing %q", plain, tt.want)
			}
			if tt.notWant != "" && strings.Contains(plain, tt.notWant) {
				t.Errorf("top bar %q should not contain %q", plain, tt.notWant)
			}
			if lipgloss.Width(got) != 120 {
				t.Errorf("width = %d, want 120", lipgloss.Width(got))
			}
		})
	}
}

func TestSidebar_AdbDownHint(t *testing.T) {
	devs := []device.Device{
		{ID: "a", Name: "A", Status: device.StatusRunning, Platform: device.PlatformIOS},
		{ID: "p", Name: "pixel_8", Status: device.StatusUnknown, Platform: device.PlatformAndroid},
	}

	s := NewSidebar()
	s.SetSize(DefaultSidebarWidth, 30)
	s.SetDevices(devs)
	if strings.Contains(ansi.Strip(s.View()), "adb server down") {
		t.Error("hint shown before the server was reported down")
	}

	s.SetAdbServer(device.AdbServerStopped)
	if !strings.Contains(ansi.Strip(s.View()), "adb server down") {
		t.Error("hint missing while the adb server is down")
	}

	// An emulator whose state cannot be read is not online.
	for _, d := range s.booted {
		if d.ID == "p" {
			t.Error("unknown-status emulator routed to the Online box")
		}
	}

	// Rows: header(iOS)=1, A=2, blank=3, hint=4. Clicking the hint selects
	// nothing new.
	s.HandleClick(2)
	s.HandleClick(4)
	if got := s.SelectedDevice(); got == nil || got.ID != "a" {
		t.Errorf("clicking the hint moved the cursor: %+v", got)
	}
}
