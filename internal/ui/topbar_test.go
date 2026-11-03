package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderTopBar(t *testing.T) {
	tests := []struct {
		name     string
		params   TopBarParams
		contains []string
	}{
		{
			name: "basic rendering",
			params: TopBarParams{
				Width:        80,
				AppVersion:   "v0.1.2",
				BootedCount:  2,
				IOSCount:     3,
				AndroidCount: 1,
			},
			contains: []string{
				"simmer",
				"v0.1.2",
				"2 online",
				"3 iOS",
				"1 Android",
			},
		},
		{
			name: "with tool versions",
			params: TopBarParams{
				Width: 80,
				ToolVersions: map[device.Platform]string{
					device.PlatformIOS:     "15.3",
					device.PlatformAndroid: "1.0.41",
				},
			},
			contains: []string{
				"xcrun 15.3",
				"adb 1.0.41",
			},
		},
		{
			name: "narrow width",
			params: TopBarParams{
				Width: 20,
			},
			contains: []string{
				"simmer",
			},
		},
		{
			name: "zero width",
			params: TopBarParams{
				Width: 0,
			},
			contains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderTopBar(tt.params)

			if tt.params.Width == 0 {
				if got != "" {
					t.Errorf("expected empty string for zero width, got %q", got)
				}
				return
			}

			// Check total visual width
			if lipgloss.Width(got) != tt.params.Width {
				t.Errorf("width = %d, want %d", lipgloss.Width(got), tt.params.Width)
			}

			// Check content
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\nGot: %q", want, got)
				}
			}
		})
	}
}

func TestRenderTopBar_InstanceIndicator(t *testing.T) {
	p := TopBarParams{Width: 120, AppVersion: "v1"}
	if strings.Contains(RenderTopBar(p), "live") {
		t.Fatal("no indicator without a View state")
	}
	if _, _, ok := TopBarInstanceSpan(p); ok {
		t.Fatal("no span without an indicator")
	}

	for state, want := range map[InstanceState]string{
		InstanceLive:       "● live",
		InstanceSuperseded: "○ not live · ^T take over",
	} {
		p.Instance = state
		line := strings.Split(ansi.Strip(RenderTopBar(p)), "\n")[0]
		if !strings.Contains(line, want) {
			t.Fatalf("top bar %q missing %q", line, want)
		}
		x0, x1, ok := TopBarInstanceSpan(p)
		runes := []rune(line)
		if !ok || string(runes[x0:x1]) != want {
			t.Fatalf("span [%d,%d) = %q, want %q", x0, x1, string(runes[x0:x1]), want)
		}
	}
}
