package ui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
)

// StatusKind selects the foreground color of the footer status message.
type StatusKind int

const (
	// StatusInfo is the default neutral color.
	StatusInfo StatusKind = iota
	// StatusOk paints the message in green (success).
	StatusOk
	// StatusWarn paints the message in yellow (caution).
	StatusWarn
	// StatusErr paints the message in red (failure).
	StatusErr
)

// FooterParams holds all data needed to render the footer.
type FooterParams struct {
	Width  int
	Status string
	Kind   StatusKind
	Help   help.Model
}

// RenderFooter renders the footer/status-bar string for the given params.
// The left side shows key binding hints via the bubbles help component.
// The right side shows a transient status message (latest user-action result).
func RenderFooter(p FooterParams) string {
	if p.Width == 0 {
		return ""
	}

	innerWidth := p.Width - 2
	hints := p.Help.View(GlobalKeys)

	left := ""
	if p.Status != "" {
		left = lipgloss.NewStyle().
			Foreground(statusColor(p.Kind)).
			Background(ColorBg).
			Render(p.Status)
	}

	gap := max(innerWidth-lipgloss.Width(left)-lipgloss.Width(hints), 0)
	// Render WITHOUT Width to prevent lipgloss word-wrapping the pre-styled ANSI
	// content at the unstyled space inside the hints string.
	// Content visual width (innerWidth) + Padding(0,1) already equals p.Width.
	gapStr := lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", gap))
	return StyleFooter.Render(left + gapStr + hints)
}

func statusColor(k StatusKind) color.Color {
	switch k {
	case StatusOk:
		return ColorOk
	case StatusWarn:
		return ColorWarn
	case StatusErr:
		return ColorErr
	default:
		return ColorFgDim
	}
}
