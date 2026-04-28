package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Tab is a single labelled tab.
type Tab struct {
	Key   string // e.g. "1"
	Label string // e.g. "Files"
}

// RenderTabs renders a horizontal tab strip of width `width`. The active tab
// is highlighted with the cyan accent; the rest are dimmed. A faint vertical
// separator follows each non-active tab.
func RenderTabs(tabs []Tab, active int, width int) string {
	if width <= 0 || len(tabs) == 0 {
		return ""
	}

	bg := lipgloss.NewStyle().Background(ColorBg)
	sepStyle := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)

	activeStyle := lipgloss.NewStyle().
		Foreground(ColorBg).
		Background(ColorAccent2).
		Bold(true).
		Padding(0, 2)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorFgDim).
		Background(ColorBg).
		Padding(0, 2)
	keyStyleInactive := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg)

	var b strings.Builder
	b.WriteString(bg.Render(" "))
	for i, t := range tabs {
		key := "[" + t.Key + "] "
		if i == active {
			// Single uniform style on the whole tab so padding cells share
			// the accent background. Nesting an inner style would punch the
			// inner segment back to ColorBg and break the block fill.
			b.WriteString(activeStyle.Render(key + t.Label))
		} else {
			b.WriteString(inactiveStyle.Render(keyStyleInactive.Render(key) + t.Label))
		}
		if i < len(tabs)-1 {
			b.WriteString(sepStyle.Render("│"))
		}
	}

	cur := lipgloss.Width(b.String())
	if cur < width {
		b.WriteString(bg.Render(strings.Repeat(" ", width-cur)))
	}
	return b.String()
}
