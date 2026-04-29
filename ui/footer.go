package ui

import (
	"image/color"
	"strings"

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
}

var footerHints = []struct{ key, verb string }{
	{"↑↓/jk", "select"},
	{"←→/hl", "pane"},
	{"b", "boot"},
	{"s", "shutdown"},
	{"1-4", "tab"},
	{"space", "load device"},
	{"?", "help"},
	{"q", "quit"},

	// [TODO] feature ideas:
	// {"r", "reboot"},
	// {"i", "install"},
	// {"/", "cmd"},
}

// RenderFooter renders the footer/status-bar string for the given params.
// The right side shows a transient status message (latest user-action result).
func RenderFooter(p FooterParams) string {
	if p.Width == 0 {
		return ""
	}

	keyStyle := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	verbStyle := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	var hintParts []string
	for _, h := range footerHints {
		hintParts = append(hintParts, keyStyle.Render(h.key)+verbStyle.Render(" "+h.verb))
	}
	left := strings.Join(hintParts, StyleFaint.Render("  "))

	right := ""
	if p.Status != "" {
		right = lipgloss.NewStyle().
			Foreground(statusColor(p.Kind)).
			Background(ColorBg).
			Render(p.Status)
	}

	innerWidth := p.Width - 2
	gap := max(innerWidth-lipgloss.Width(left)-lipgloss.Width(right), 0)

	return StyleFooter.Width(p.Width).Render(left + strings.Repeat(" ", gap) + right)
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
