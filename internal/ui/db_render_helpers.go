package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// dbRenderHelpers provides shared row-building utilities used by all DB viewer panes.
// Each pane holds one instance constructed at init time.
type dbRenderHelpers struct {
	bg  lipgloss.Style
	sep lipgloss.Style
}

func newDBRenderHelpers() dbRenderHelpers {
	return dbRenderHelpers{
		bg:  lipgloss.NewStyle().Background(ColorBg),
		sep: lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg),
	}
}

// BlankN returns n background-colored spaces. Returns "" for n ≤ 0.
func (h dbRenderHelpers) BlankN(n int) string {
	if n <= 0 {
		return ""
	}
	return h.bg.Render(strings.Repeat(" ", n))
}

// FillTo pads s with background spaces until its visible width equals width.
// If s is already wider, s is returned unchanged.
func (h dbRenderHelpers) FillTo(s string, width int) string {
	need := width - lipgloss.Width(s)
	if need <= 0 {
		return s
	}
	return s + h.BlankN(need)
}

// Sep returns a full-width horizontal rule in the border color.
func (h dbRenderHelpers) Sep(width int) string {
	return h.sep.Render(strings.Repeat("─", width))
}
