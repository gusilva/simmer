package ui

import (
	"charm.land/lipgloss/v2"
	"simmer/internal/theme"
)

// Color tokens — defined in internal/theme; re-exported here so ui sub-files
// can use the short names without importing theme directly.
var (
	ColorBg    = theme.ColorBg
	ColorBgDim = theme.ColorBgDim

	ColorBorder   = theme.ColorBorder
	ColorBorderHi = theme.ColorBorderHi

	ColorFg      = theme.ColorFg
	ColorFgDim   = theme.ColorFgDim
	ColorFgFaint = theme.ColorFgFaint

	ColorAccent  = theme.ColorAccent
	ColorAccent2 = theme.ColorAccent2

	ColorOk   = theme.ColorOk
	ColorErr  = theme.ColorErr
	ColorInfo = theme.ColorInfo
	ColorWarn = theme.ColorWarn

	ColorOrange = theme.ColorOrange
	ColorPink   = theme.ColorPink

	ColorTableHeader = theme.ColorTableHeader
	ColorTableSelBg  = theme.ColorTableSelBg

	ColorIOS     = theme.ColorIOS
	ColorAndroid = theme.ColorAndroid
)

// Shared styles
var (
	StyleDoc = lipgloss.NewStyle().Margin(0).Background(ColorBg)

	StyleTopBar = lipgloss.NewStyle().
			Background(ColorBg).
			Padding(0, 1).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottomForeground(ColorBorder).
			BorderBottomBackground(ColorBg)

	StyleFooter = lipgloss.NewStyle().
			Background(ColorBg).
			Padding(0, 1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTopForeground(ColorBorder).
			BorderTopBackground(ColorBg)

	StyleBrand = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Background(ColorBg).
			Bold(true)

	StyleFaint = lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Background(ColorBg)
)
