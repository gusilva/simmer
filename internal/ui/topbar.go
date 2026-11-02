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
	AdbServer    device.AdbServerState
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
		metaParts = append(metaParts, StyleFaint.Render("xcrun "+v))
	}
	if v := p.ToolVersions[device.PlatformAndroid]; v != "" {
		metaParts = append(metaParts, StyleFaint.Render("adb "+v)+adbServerBadge(p.AdbServer))
	}

	right := strings.Join(metaParts, sep)

	innerWidth := p.Width - 2
	gap := max(innerWidth-lipgloss.Width(left)-lipgloss.Width(right), 0)

	return StyleTopBar.Width(p.Width).Render(left + strings.Repeat(" ", gap) + right)
}

// adbServerBadge marks the adb version with whether its server is running.
// The version alone says nothing about that — `adb version` answers with no
// server — so without the badge a stopped server looks healthy.
func adbServerBadge(s device.AdbServerState) string {
	switch s {
	case device.AdbServerRunning:
		return StyleFaint.Render(" ") + lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Render("●")
	case device.AdbServerStopped:
		return StyleFaint.Render(" ") + lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg).Render("● server down")
	default:
		return ""
	}
}
