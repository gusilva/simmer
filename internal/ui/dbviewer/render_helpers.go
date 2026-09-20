package dbviewer

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"simmer/internal/theme"
)

// renderHelpers provides shared row-building utilities used by all DB viewer panes.
// Each pane holds one instance constructed at init time.
type renderHelpers struct {
	bg  lipgloss.Style
	sep lipgloss.Style
}

func newRenderHelpers() renderHelpers {
	return renderHelpers{
		bg:  lipgloss.NewStyle().Background(theme.ColorBg),
		sep: lipgloss.NewStyle().Foreground(theme.ColorBorder).Background(theme.ColorBg),
	}
}

// WithSepColor returns a copy of h whose separator rule is tinted c. Panes
// call this at the top of Rows() with theme.ColorBorderHi when they hold
// focus, so the pane's own divider lights up instead of dimming the whole
// pane's background.
func (h renderHelpers) WithSepColor(c color.Color) renderHelpers {
	h.sep = lipgloss.NewStyle().Foreground(c).Background(theme.ColorBg)
	return h
}

// BlankN returns n background-colored spaces. Returns "" for n ≤ 0.
func (h renderHelpers) BlankN(n int) string {
	if n <= 0 {
		return ""
	}
	return h.bg.Render(strings.Repeat(" ", n))
}

// FillTo pads s with background spaces until its visible width equals width.
// If s is already wider, s is returned unchanged.
func (h renderHelpers) FillTo(s string, width int) string {
	need := width - lipgloss.Width(s)
	if need <= 0 {
		return s
	}
	return s + h.BlankN(need)
}

// ExactWidth returns a string whose visible width is exactly width:
// pads with background spaces if short, clips with MaxWidth if long.
func (h renderHelpers) ExactWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w < width {
		return s + h.BlankN(width-w)
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

// Sep returns a full-width horizontal rule in the border color.
func (h renderHelpers) Sep(width int) string {
	return h.sep.Render(strings.Repeat("─", width))
}
