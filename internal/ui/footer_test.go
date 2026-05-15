package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderFooter(t *testing.T) {
	tests := []struct {
		name   string
		params FooterParams
	}{
		{
			name: "Status info",
			params: FooterParams{
				Width:  100,
				Status: "Ready",
				Kind:   StatusInfo,
				Help:   NewHelpModel(),
			},
		},
		{
			name: "Status success",
			params: FooterParams{
				Width:  100,
				Status: "Booted iPhone 15",
				Kind:   StatusOk,
				Help:   NewHelpModel(),
			},
		},
		{
			name: "Status warn",
			params: FooterParams{
				Width:  100,
				Status: "Warning: low space",
				Kind:   StatusWarn,
				Help:   NewHelpModel(),
			},
		},
		{
			name: "Status error",
			params: FooterParams{
				Width:  100,
				Status: "Failed to boot",
				Kind:   StatusErr,
				Help:   NewHelpModel(),
			},
		},
		{
			name: "Narrow footer",
			params: FooterParams{
				Width:  40,
				Status: "Busy",
				Help:   NewHelpModel(),
			},
		},
		{
			name: "Zero width",
			params: FooterParams{
				Width: 0,
				Help:  NewHelpModel(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderFooter(tt.params)

			if tt.params.Width == 0 {
				if got != "" {
					t.Errorf("expected empty string for zero width")
				}
				return
			}

			// Check width
			if w := lipgloss.Width(got); w != tt.params.Width {
				t.Errorf("width = %d, want %d", w, tt.params.Width)
			}

			// Check status presence
			if tt.params.Status != "" && !strings.Contains(got, tt.params.Status) {
				t.Errorf("status %q not found in output", tt.params.Status)
			}

			// Check that the active hint is rendered (only ? is active in GlobalKeys)
			if tt.params.Width > 40 && !strings.Contains(got, "?") {
				t.Errorf("help hint %q not found in output", "?")
			}

			// Check single-line content (border top + one content row = 2 lines total)
			lines := strings.Split(got, "\n")
			if len(lines) != 2 {
				t.Errorf("footer rendered %d lines, want 2", len(lines))
			}
		})
	}
}
