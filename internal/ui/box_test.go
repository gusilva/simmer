package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderBox(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		badge       string
		content     string
		footer      string
		width       int
		height      int
		focused     bool
		keyHint     string
		bottomBadge string
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
		{
			name:    "keyHint in top border",
			title:   "Online",
			content: "content",
			width:   36,
			focused: true,
			keyHint: "1",
		},
		{
			name:        "bottomBadge right-aligned",
			title:       "Offline",
			content:     "item",
			width:       36,
			bottomBadge: "1 of 3",
		},
		{
			name:        "keyHint and bottomBadge narrow box no panic",
			title:       "T",
			content:     "c",
			width:       12,
			keyHint:     "1",
			bottomBadge: "1 of 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderBox(tt.title, tt.badge, tt.content, tt.footer, tt.width, tt.height, tt.focused, tt.keyHint, tt.bottomBadge)

			if tt.width < 6 {
				if got != "" {
					t.Errorf("expected empty string for narrow width, got %q", got)
				}
				return
			}

			// Every line must match the box width.
			lines := strings.Split(got, "\n")
			for i, line := range lines {
				if w := lipgloss.Width(line); w != tt.width {
					t.Errorf("line %d width = %d, want %d", i, w, tt.width)
				}
			}

			// Fixed height check.
			if tt.height > 0 {
				if len(lines) != tt.height {
					t.Errorf("height = %d, want %d", len(lines), tt.height)
				}
			}

			if tt.title != "" && !strings.Contains(got, tt.title[:min(len(tt.title), 3)]) {
				t.Errorf("title not found in output")
			}
			if tt.footer != "" && !strings.Contains(got, tt.footer) {
				t.Errorf("footer not found in output")
			}

			// keyHint must appear in the top border.
			if tt.keyHint != "" && !strings.Contains(lines[0], "["+tt.keyHint+"]") {
				t.Errorf("keyHint [%s] not found in top border: %q", tt.keyHint, lines[0])
			}

			// bottomBadge must appear in the bottom border when it fits.
			if tt.bottomBadge != "" {
				bottom := lines[len(lines)-1]
				badgeW := lipgloss.Width(tt.bottomBadge)
				inner := tt.width - 2
				if inner-badgeW-2 >= 0 {
					if !strings.Contains(bottom, tt.bottomBadge) {
						t.Errorf("bottomBadge %q not found in bottom border: %q", tt.bottomBadge, bottom)
					}
				}
			}
		})
	}
}
