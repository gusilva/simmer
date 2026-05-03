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
			},
		},
		{
			name: "Status success",
			params: FooterParams{
				Width:  100,
				Status: "Booted iPhone 15",
				Kind:   StatusOk,
			},
		},
		{
			name: "Status error",
			params: FooterParams{
				Width:  100,
				Status: "Failed to boot",
				Kind:   StatusErr,
			},
		},
		{
			name: "Narrow footer",
			params: FooterParams{
				Width:  40,
				Status: "Busy",
			},
		},
		{
			name: "Zero width",
			params: FooterParams{
				Width: 0,
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

			// Check help hints (just a few key ones)
			hints := []string{"select", "quit", "boot"}
			for _, h := range hints {
				if tt.params.Width > 80 && !strings.Contains(got, h) {
					t.Errorf("hint %q not found in output", h)
				}
			}
		})
	}
}
