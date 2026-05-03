package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderBox(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		badge   string
		content string
		footer  string
		width   int
		height  int
		focused bool
	}{
		{
			name:    "Basic box",
			title:   "Title",
			content: "Line 1\nLine 2",
			width:   20,
			height:  0,
			focused: true,
		},
		{
			name:    "Box with badge and footer",
			title:   "Title",
			badge:   "99",
			content: "Content",
			footer:  "Footer",
			width:   30,
			height:  10,
			focused: false,
		},
		{
			name:    "Narrow box truncation",
			title:   "Very Long Title That Should Be Truncated",
			content: "Very Long Content Line That Should Be Truncated",
			width:   10,
			height:  0,
		},
		{
			name:  "Too narrow box",
			width: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderBox(tt.title, tt.badge, tt.content, tt.footer, tt.width, tt.height, tt.focused)

			if tt.width < 6 {
				if got != "" {
					t.Errorf("expected empty string for narrow width, got %q", got)
				}
				return
			}

			// Check width
			lines := strings.Split(got, "\n")
			for i, line := range lines {
				if w := lipgloss.Width(line); w != tt.width {
					t.Errorf("line %d width = %d, want %d", i, w, tt.width)
				}
			}

			// Check height if fixed
			if tt.height > 0 {
				if len(lines) != tt.height {
					t.Errorf("height = %d, want %d", len(lines), tt.height)
				}
			}

			// Check content
			if tt.title != "" && !strings.Contains(got, tt.title[:min(len(tt.title), 3)]) {
				t.Errorf("title not found in output")
			}
			if tt.footer != "" && !strings.Contains(got, tt.footer) {
				t.Errorf("footer not found in output")
			}
		})
	}
}
