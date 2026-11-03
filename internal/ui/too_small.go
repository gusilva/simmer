package ui

import (
	"strconv"

	"charm.land/lipgloss/v2"
)

// RenderTooSmall is drawn in place of the whole TUI while the terminal is
// below the minimum size the layout is built for. Each current dimension is
// coloured by whether it already meets its minimum, so the user can see
// which way to resize.
func RenderTooSmall(width, height, minWidth, minHeight int) string {
	bold := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	dimension := func(cur, need int) string {
		c := ColorOk
		if cur < need {
			c = ColorErr
		}
		return lipgloss.NewStyle().Foreground(c).Background(ColorBg).Bold(true).Render(strconv.Itoa(cur))
	}

	msg := lipgloss.JoinVertical(lipgloss.Center,
		bold.Render("Terminal size too small:"),
		dim.Render("Width = ")+dimension(width, minWidth)+dim.Render(" Height = ")+dimension(height, minHeight),
		"",
		bold.Render("Needed for simmer:"),
		dim.Render("Width = "+strconv.Itoa(minWidth)+" Height = "+strconv.Itoa(minHeight)),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(ColorBg)))
}
