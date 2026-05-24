package ui

import (
	"fmt"
	"strings"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

// TopBarParams holds all data needed to render the top bar.
type TopBarParams struct {
	Width        int
	AppVersion   string
	BootedCount  int
	IOSCount     int
	AndroidCount int
	ToolVersions map[device.Platform]string
}

// RenderTopBar renders the top bar string for the given params.
func RenderTopBar(p TopBarParams) string {
	if p.Width == 0 {
		return ""
	}

	sep := StyleFaint.Render(" · ")

	left := StyleBrand.Render("simmer") +
		StyleFaint.Render(" "+p.AppVersion) +
		sep +
		lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Render("●") +
		StyleDim.Render(fmt.Sprintf(" %d online", p.BootedCount)) +
		StyleFaint.Render("  ") +
		lipgloss.NewStyle().Foreground(ColorIOS).Background(ColorBg).Render("⌘") +
		StyleDim.Render(fmt.Sprintf(" %d iOS", p.IOSCount)) +
		StyleFaint.Render("  ") +
		lipgloss.NewStyle().Foreground(ColorAndroid).Background(ColorBg).Render("⛯") +
		StyleDim.Render(fmt.Sprintf(" %d Android", p.AndroidCount))

	var metaParts []string
	if v := p.ToolVersions[device.PlatformIOS]; v != "" {
		metaParts = append(metaParts, "xcrun "+v)
	}
	if v := p.ToolVersions[device.PlatformAndroid]; v != "" {
		metaParts = append(metaParts, "adb "+v)
	}

	right := StyleFaint.Render(strings.Join(metaParts, " · "))

	innerWidth := p.Width - 2
	gap := max(innerWidth-lipgloss.Width(left)-lipgloss.Width(right), 0)

	return StyleTopBar.Width(p.Width).Render(left + strings.Repeat(" ", gap) + right)
}
