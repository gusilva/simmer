package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderTabs(t *testing.T) {
	tabs := []Tab{
		{Key: "1", Label: "Files"},
		{Key: "2", Label: "Logs"},
		{Key: "3", Label: "SQLite"},
	}

	tests := []struct {
		name   string
		active int
		width  int
	}{
		{"First tab active", 0, 80},
		{"Middle tab active", 1, 80},
		{"Last tab active", 2, 80},
		{"Narrow width", 1, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderTabs(tabs, tt.active, tt.width)

			// Check total visual width (ensures minimum width)
			gotW := lipgloss.Width(got)
			if gotW < tt.width {
				t.Errorf("width = %d, want at least %d", gotW, tt.width)
			}

			// Check if active tab label is present
			if !strings.Contains(got, tabs[tt.active].Label) {
				t.Errorf("missing active tab label %q", tabs[tt.active].Label)
			}
		})
	}

	t.Run("Empty tabs", func(t *testing.T) {
		got := RenderTabs([]Tab{}, 0, 80)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("Zero width", func(t *testing.T) {
		got := RenderTabs(tabs, 0, 0)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}
