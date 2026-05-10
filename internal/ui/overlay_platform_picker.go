package ui

import (
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── PlatformPickerModal ──────────────────────────────────────────────────────

const platformPickerW = 46

var platformPickerItems = []struct {
	label    string
	platform device.Platform
}{
	{"iOS Simulator", device.PlatformIOS},
	{"Android Emulator", device.PlatformAndroid},
}

// PlatformPickerModal lets the user choose iOS or Android before creating a device.
type PlatformPickerModal struct {
	idx int
}

// NewPlatformPickerModal returns a picker with iOS selected by default.
func NewPlatformPickerModal() PlatformPickerModal { return PlatformPickerModal{} }

func (m PlatformPickerModal) Update(msg tea.Msg) (PlatformPickerModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "up", "k":
		if m.idx > 0 {
			m.idx--
		}
	case "down", "j":
		if m.idx < len(platformPickerItems)-1 {
			m.idx++
		}
	case "enter":
		p := platformPickerItems[m.idx].platform
		return m, func() tea.Msg { return ConfirmPlatformPickerMsg{Platform: p} }
	}
	return m, nil
}

func (m PlatformPickerModal) View() string {
	innerW := platformPickerW - 6

	title := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true).Render("Add Device")
	lbl := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render("Select platform:")
	itemSel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
	itemNorm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(lbl)
	b.WriteString("\n\n")

	for i, item := range platformPickerItems {
		name := item.label
		if i == m.idx {
			pad := strings.Repeat(" ", max(innerW-3-lipgloss.Width(name), 1))
			b.WriteString(itemSel.Render(" ▸ " + name + pad))
		} else {
			b.WriteString(itemNorm.Render("   " + name))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	sep := hintVerb.Render("  ")
	hints := strings.Join([]string{
		hintKey.Render("↑↓") + hintVerb.Render(" select"),
		hintKey.Render("Enter") + hintVerb.Render(" confirm"),
		hintKey.Render("Esc") + hintVerb.Render(" cancel"),
	}, sep)
	b.WriteString(hints)

	return box.Width(platformPickerW).Render(b.String())
}
