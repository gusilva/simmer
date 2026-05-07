package ui

import "charm.land/lipgloss/v2"

// dbViewerStyles holds all pre-built styles for the DB viewer modal.
// Constructed once at NewDBViewerModal and reused every frame.
type dbViewerStyles struct {
	Outer lipgloss.Style
	Inner lipgloss.Style
	Bg    lipgloss.Style

	// Title bar
	Icon  lipgloss.Style
	Label lipgloss.Style
	Dot   lipgloss.Style
	Dim   lipgloss.Style
	Faint lipgloss.Style
	Fg    lipgloss.Style

	// Status bar
	Key             lipgloss.Style
	Val             lipgloss.Style
	Ok              lipgloss.Style
	ModeBadgeInsert lipgloss.Style
	ModeBadgeNormal lipgloss.Style
}

func newDBViewerStyles() dbViewerStyles {
	return dbViewerStyles{
		Outer: lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg),
		Inner: lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg),
		Bg:    lipgloss.NewStyle().Background(ColorBg),

		Icon:  lipgloss.NewStyle().Foreground(ColorAccent2).Background(ColorBg).PaddingLeft(1),
		Label: lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true),
		Dot:   lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg),
		Dim:   lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg),
		Faint: lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).PaddingRight(1),
		Fg:    lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg),

		Key: lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true),
		Val: lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg),
		Ok:  lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg),

		ModeBadgeInsert: lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorWarn).Bold(true).Padding(0, 1),
		ModeBadgeNormal: lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorAccent).Bold(true).Padding(0, 1),
	}
}
