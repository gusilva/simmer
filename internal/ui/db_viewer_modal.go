package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ShowDBViewerMsg opens the database viewer overlay.
type ShowDBViewerMsg struct{}

type dbViewerFocus int

const (
	dbFocusSidebar dbViewerFocus = iota
	dbFocusQuery
	dbFocusResults
)

// DBViewerModal is the full-screen database viewer overlay.
type DBViewerModal struct {
	width  int
	height int
	focus  dbViewerFocus
}

func NewDBViewerModal() DBViewerModal { return DBViewerModal{focus: dbFocusSidebar} }

func (m *DBViewerModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m DBViewerModal) Update(msg tea.Msg) (DBViewerModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc", "q":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "tab":
		m.focus = (m.focus + 1) % 3
	case "shift+tab":
		m.focus = (m.focus + 2) % 3
	}
	return m, nil
}

func (m DBViewerModal) modalW() int {
	if m.width <= 4 {
		return 30
	}
	return m.width - 4
}

func (m DBViewerModal) modalH() int {
	if m.height <= 4 {
		return 10
	}
	return m.height - 4
}

// View renders the DB viewer as an ANSI string for overlay placement.
func (m DBViewerModal) View() string {
	mw := m.modalW()
	mh := m.modalH()
	innerW := mw - 2
	innerH := mh - 2

	// Sidebar occupies a fixed left column; a │ divider separates it from the
	// right pane. The divider itself is 1 char wide.
	const sidebarW = 24
	const divW = 1
	rightW := innerW - sidebarW - divW

	// The horizontal rule inside the right pane splits query (top) from results.
	queryH := max(innerH/3, 3)
	// dividerRow is the 0-based index of the horizontal-rule row.
	dividerRow := queryH

	// ── styles ──
	outerS := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg)
	innerS := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)
	bgS := lipgloss.NewStyle().Background(ColorBg)

	blank := func(n int) string { return bgS.Render(strings.Repeat(" ", n)) }

	var sb strings.Builder

	// Top border — ┬ is part of the outer border (accent colour), not the inner divider.
	sb.WriteString(outerS.Render("╭" + strings.Repeat("─", sidebarW) + "┬" + strings.Repeat("─", rightW) + "╮"))
	sb.WriteByte('\n')

	// Content rows.
	for row := range innerH {
		sb.WriteString(outerS.Render("│"))
		sb.WriteString(blank(sidebarW))

		if row == dividerRow {
			// ┼ is a purely inner intersection (dim); ┤ touches the outer border (accent).
			sb.WriteString(innerS.Render("├" + strings.Repeat("─", rightW)))
			sb.WriteString(outerS.Render("│"))
		} else {
			sb.WriteString(innerS.Render("│"))
			sb.WriteString(blank(rightW))
			sb.WriteString(outerS.Render("│"))
		}

		sb.WriteByte('\n')
	}

	// Bottom border — ┴ is part of the outer border (accent colour).
	sb.WriteString(outerS.Render("╰" + strings.Repeat("─", sidebarW) + "┴" + strings.Repeat("─", rightW) + "╯"))

	return sb.String()
}

// ScrimView returns a full-terminal-size overlay that simulates opacity.
// True alpha is not possible in terminals; ░ shade characters bleed the
// background colour through the pattern, approximating a semi-transparent dim.
func (m DBViewerModal) ScrimView() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	// ░ foreground = very dark (near-invisible), background = app bg.
	// The shade pattern lets the app bg show through while dimming the content.
	scrimS := lipgloss.NewStyle().Foreground(ColorBgDim).Background(ColorBg)
	row := scrimS.Render(strings.Repeat("░", m.width))
	rows := make([]string, m.height)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}
