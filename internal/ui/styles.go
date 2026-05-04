package ui

import "charm.land/lipgloss/v2"

// Color tokens
var (
	ColorBg    = lipgloss.Color("#0d0e16")
	ColorBgDim = lipgloss.Color("#07080f") // scrim / dimmed background overlay

	ColorBorder   = lipgloss.Color("#2a2c3a")
	ColorBorderHi = lipgloss.Color("#cba6f7")

	ColorFg      = lipgloss.Color("#e6e6f0")
	ColorFgDim   = lipgloss.Color("#9da0b3")
	ColorFgFaint = lipgloss.Color("#5a5d72")

	ColorAccent  = lipgloss.Color("#cba6f7")
	ColorAccent2 = lipgloss.Color("#89dceb")

	ColorOk   = lipgloss.Color("#a6e3a1")
	ColorErr  = lipgloss.Color("#f38ba8")
	ColorInfo = lipgloss.Color("#89b4fa")
	ColorWarn = lipgloss.Color("#f9e2af")

	ColorIOS     = lipgloss.Color("#89dceb")
	ColorAndroid = lipgloss.Color("#a6e3a1")
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
