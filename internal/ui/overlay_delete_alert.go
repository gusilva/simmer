package ui

import (
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── DeleteSimulatorAlert ─────────────────────────────────────────────────────

const deleteAlertW = 50

// DeleteSimulatorAlert is a confirmation dialog before deleting a simulator.
type DeleteSimulatorAlert struct {
	device       device.Device
	confirmFocus bool // false = Cancel focused, true = Delete focused
}

// NewDeleteSimulatorAlert returns an alert with Cancel focused by default.
func NewDeleteSimulatorAlert(dev device.Device) DeleteSimulatorAlert {
	return DeleteSimulatorAlert{device: dev}
}

// Update handles key events for the delete confirmation.
func (m DeleteSimulatorAlert) Update(msg tea.Msg) (DeleteSimulatorAlert, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc", "n", "N":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "y", "Y":
		dev := m.device
		return m, func() tea.Msg { return ConfirmDeleteSimulatorMsg{Device: dev} }
	case "enter":
		if m.confirmFocus {
			dev := m.device
			return m, func() tea.Msg { return ConfirmDeleteSimulatorMsg{Device: dev} }
		}
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "tab", "left", "h", "right", "l":
		m.confirmFocus = !m.confirmFocus
	}
	return m, nil
}

// View renders the alert box as an ANSI string for overlay placement.
func (m DeleteSimulatorAlert) View() string {
	innerW := deleteAlertW - 6

	norm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	// Buttons: always rounded-border so both are 3 lines tall and align correctly.
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

	name := truncateName(m.device.Name, innerW-4)

	var b strings.Builder
	titleStr := "Delete Simulator"
	if m.device.Platform == device.PlatformAndroid {
		titleStr = "Delete Emulator"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg).Bold(true).Render(titleStr))
	b.WriteString("\n\n")
	b.WriteString(norm.Render(`"` + name + `"`))
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

	return box.Width(deleteAlertW).Render(b.String())
}
