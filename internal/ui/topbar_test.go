package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
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
				"2 booted",
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
