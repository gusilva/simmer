package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── ActionMenuModal ───────────────────────────────────────────────────────

const actionMenuW = 40

// ActionMenuItem is one selectable row in an ActionMenuModal. Cmd is the
// exact tea.Cmd the equivalent keyboard shortcut already runs — the menu is
// purely a discoverability layer, not a new code path.
type ActionMenuItem struct {
	Label string
	Cmd   tea.Cmd
}

// ActionMenuModal lists the actions available for a double-clicked row.
type ActionMenuModal struct {
	title string
	items []ActionMenuItem
	idx   int
}

// NewActionMenuModal returns a menu titled `title` listing `items`.
func NewActionMenuModal(title string, items []ActionMenuItem) ActionMenuModal {
	return ActionMenuModal{title: title, items: items}
}

// ActionMenuSelectedMsg carries the chosen item's Cmd for the parent to run.
type ActionMenuSelectedMsg struct{ Cmd tea.Cmd }

func (m ActionMenuModal) Update(msg tea.Msg) (ActionMenuModal, tea.Cmd) {
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
		if m.idx < len(m.items)-1 {
			m.idx++
		}
	case "enter":
		if m.idx < len(m.items) {
			cmd := m.items[m.idx].Cmd
			return m, func() tea.Msg { return ActionMenuSelectedMsg{Cmd: cmd} }
		}
	}
	return m, nil
}

func (m ActionMenuModal) View() string {
	innerW := actionMenuW - 6

	title := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true).Render(m.title)
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

	for i, item := range m.items {
		name := item.Label
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

	return box.Width(actionMenuW).Render(b.String())
}
