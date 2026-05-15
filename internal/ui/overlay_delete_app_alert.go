package ui

import (
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── DeleteAppAlert ────────────────────────────────────────────────────────────

const deleteAppAlertW = 50

// DeleteAppAlert is a confirmation dialog before uninstalling an app.
type DeleteAppAlert struct {
	device       device.Device
	app          device.App
	confirmFocus bool // false = Cancel focused, true = Delete focused
}

// NewDeleteAppAlert returns an alert with Cancel focused by default.
func NewDeleteAppAlert(dev device.Device, app device.App) DeleteAppAlert {
	return DeleteAppAlert{device: dev, app: app}
}

// Update handles key events for the delete app confirmation.
func (m DeleteAppAlert) Update(msg tea.Msg) (DeleteAppAlert, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc", "n", "N":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "y", "Y":
		dev, app := m.device, m.app
		return m, func() tea.Msg { return ConfirmDeleteAppMsg{Device: dev, App: app} }
	case "enter":
		if m.confirmFocus {
			dev, app := m.device, m.app
			return m, func() tea.Msg { return ConfirmDeleteAppMsg{Device: dev, App: app} }
		}
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "tab", "left", "h", "right", "l":
		m.confirmFocus = !m.confirmFocus
	}
	return m, nil
}

// View renders the alert box as an ANSI string for overlay placement.
func (m DeleteAppAlert) View() string {
	innerW := deleteAppAlertW - 6

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
	deleteStyle := lipgloss.NewStyle().
		Foreground(ColorErr).Background(ColorBg).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).
		Padding(0, 2)
	deleteFocusStyle := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorErr).Bold(true).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorErr).
		Padding(0, 2)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorErr).
		Background(ColorBg).
		Padding(1, 2)

	appName := truncateName(m.app.Label(), innerW-4)
	bundleID := truncateName(m.app.BundleID, innerW-4)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg).Bold(true).Render("Delete App"))
	b.WriteString("\n\n")
	b.WriteString(norm.Render(`"` + appName + `"`))
	b.WriteString("\n")
	b.WriteString(faint.Render(bundleID))
	b.WriteString("\n")
	b.WriteString(faint.Render("This cannot be undone."))
	b.WriteString("\n\n")

	cancelBtn := cancelStyle.Render("Cancel")
	deleteBtn := deleteStyle.Render("Delete")
	if !m.confirmFocus {
		cancelBtn = cancelFocusStyle.Render("Cancel")
	} else {
		deleteBtn = deleteFocusStyle.Render("Delete")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "   ", deleteBtn)
	b.WriteString(lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Background(ColorBg).Render(buttons))
	b.WriteString("\n\n")

	sep := hintVerb.Render("  ")
	hints := strings.Join([]string{
		hintKey.Render("Tab") + hintVerb.Render(" switch"),
		hintKey.Render("y") + hintVerb.Render(" delete"),
		hintKey.Render("Esc") + hintVerb.Render(" cancel"),
	}, sep)
	b.WriteString(hints)

	return box.Width(deleteAppAlertW).Render(b.String())
}
