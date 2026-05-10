package dbviewer

import (
	"charm.land/lipgloss/v2"
	"simmer/internal/theme"
)

// viewerStyles holds all pre-built styles for the DB viewer modal.
// Constructed once at New and reused every frame.
type viewerStyles struct {
	Outer lipgloss.Style
	Inner lipgloss.Style
	Bg    lipgloss.Style

	Icon  lipgloss.Style
	Label lipgloss.Style
	Dot   lipgloss.Style
	Dim   lipgloss.Style
	Faint lipgloss.Style
	Fg    lipgloss.Style

	Key             lipgloss.Style
	Val             lipgloss.Style
	Ok              lipgloss.Style
	Err             lipgloss.Style
	Warn            lipgloss.Style
	ModeBadgeInsert lipgloss.Style
	ModeBadgeNormal lipgloss.Style
}

func newViewerStyles() viewerStyles {
	return viewerStyles{
		Outer: lipgloss.NewStyle().Foreground(theme.ColorBorderHi).Background(theme.ColorBg),
		Inner: lipgloss.NewStyle().Foreground(theme.ColorBorder).Background(theme.ColorBg),
		Bg:    lipgloss.NewStyle().Background(theme.ColorBg),

		Icon:  lipgloss.NewStyle().Foreground(theme.ColorAccent2).Background(theme.ColorBg).PaddingLeft(1),
		Label: lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true),
		Dot:   lipgloss.NewStyle().Foreground(theme.ColorOk).Background(theme.ColorBg),
		Dim:   lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg),
		Faint: lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg).PaddingRight(1),
		Fg:    lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg),

		Key:  lipgloss.NewStyle().Foreground(theme.ColorBorderHi).Background(theme.ColorBg).Bold(true),
		Val:  lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg),
		Ok:   lipgloss.NewStyle().Foreground(theme.ColorOk).Background(theme.ColorBg),
		Err:  lipgloss.NewStyle().Foreground(theme.ColorErr).Background(theme.ColorBg).Bold(true),
		Warn: lipgloss.NewStyle().Foreground(theme.ColorWarn).Background(theme.ColorBg),

		ModeBadgeInsert: lipgloss.NewStyle().
			Foreground(theme.ColorBg).Background(theme.ColorWarn).Bold(true).Padding(0, 1),
		ModeBadgeNormal: lipgloss.NewStyle().
			Foreground(theme.ColorBg).Background(theme.ColorAccent).Bold(true).Padding(0, 1),
	}
}
