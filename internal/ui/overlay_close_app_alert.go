package ui

import (
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── CloseAppAlert ─────────────────────────────────────────────────────────────

const closeAppAlertW = 50

// CloseAppAlert is a confirmation dialog before terminating a running app.
type CloseAppAlert struct {
	device       device.Device
	app          device.App
	confirmFocus bool // false = Cancel focused, true = Close focused
}

// NewCloseAppAlert returns an alert with Cancel focused by default.
func NewCloseAppAlert(dev device.Device, app device.App) CloseAppAlert {
	return CloseAppAlert{device: dev, app: app}
}

// Update handles key events for the close app confirmation.
func (m CloseAppAlert) Update(msg tea.Msg) (CloseAppAlert, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc", "n", "N":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "y", "Y", "c", "C":
		dev, app := m.device, m.app
		return m, func() tea.Msg { return ConfirmCloseAppMsg{Device: dev, App: app} }
	case "enter":
		if m.confirmFocus {
			dev, app := m.device, m.app
			return m, func() tea.Msg { return ConfirmCloseAppMsg{Device: dev, App: app} }
		}
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "tab", "left", "h", "right", "l":
		m.confirmFocus = !m.confirmFocus
	}
	return m, nil
}

// View renders the alert box as an ANSI string for overlay placement.
func (m CloseAppAlert) View() string {
	innerW := closeAppAlertW - 6

	norm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	cancelStyle := lipgloss.NewStyle().
		Foreground(ColorFgDim).Background(ColorBg).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).
		Padding(0, 2)
	cancelFocusStyle := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorFgDim).Bold(true).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorFgDim).
		Padding(0, 2)
	closeStyle := lipgloss.NewStyle().
		Foreground(ColorWarn).Background(ColorBg).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).
		Padding(0, 2)
	closeFocusStyle := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorWarn).Bold(true).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorWarn).
		Padding(0, 2)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarn).
		Background(ColorBg).
		Padding(1, 2)

	appName := truncateName(m.app.Label(), innerW-4)
	bundleID := truncateName(m.app.BundleID, innerW-4)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ColorWarn).Background(ColorBg).Bold(true).Render("Close App"))
	b.WriteString("\n\n")
	b.WriteString(norm.Render(`"` + appName + `"`))
	b.WriteString("\n")
	b.WriteString(faint.Render(bundleID))
	b.WriteString("\n\n")

	cancelBtn := cancelStyle.Render("Cancel")
	closeBtn := closeStyle.Render("Close")
	if !m.confirmFocus {
		cancelBtn = cancelFocusStyle.Render("Cancel")
	} else {
		closeBtn = closeFocusStyle.Render("Close")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "   ", closeBtn)
	b.WriteString(lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Background(ColorBg).Render(buttons))
	b.WriteString("\n\n")

	sep := hintVerb.Render("  ")
	hints := strings.Join([]string{
		hintKey.Render("Tab") + hintVerb.Render(" switch"),
		hintKey.Render("c") + hintVerb.Render(" close"),
		hintKey.Render("Esc") + hintVerb.Render(" cancel"),
	}, sep)
	b.WriteString(hints)

	return box.Width(closeAppAlertW).Render(b.String())
}
