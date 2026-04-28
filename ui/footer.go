package ui

import (
	"strings"

	"simmer/pkg/device"

	"charm.land/lipgloss/v2"
)

// FooterDevice holds display data for the selected device summary.
type FooterDevice struct {
	Name     string
	Platform device.Platform
	Status   device.Status
}

// FooterParams holds all data needed to render the footer.
type FooterParams struct {
	Width        int
	ActiveDevice *FooterDevice
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

	var right string
	if p.ActiveDevice != nil {
		dotColor := ColorFgFaint
		if p.ActiveDevice.Status == device.StatusRunning {
			dotColor = ColorOk
		}
		dot := lipgloss.NewStyle().Foreground(dotColor).Background(ColorBg).Render("●")
		name := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(p.ActiveDevice.Name)

		platformColor := ColorIOS
		platformGlyph := ""
		if p.ActiveDevice.Platform == device.PlatformAndroid {
			platformColor = ColorAndroid
			platformGlyph = "▲"
		}
		glyph := lipgloss.NewStyle().Foreground(platformColor).Background(ColorBg).Render(platformGlyph)

		parts := dot + " " + glyph + " " + name
		right = parts
	}

	innerWidth := p.Width - 2
	gap := max(innerWidth-lipgloss.Width(left)-lipgloss.Width(right), 0)

	return StyleFooter.Width(p.Width).Render(left + strings.Repeat(" ", gap) + right)
}
