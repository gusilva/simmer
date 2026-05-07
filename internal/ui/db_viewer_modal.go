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
	width       int
	height      int
	focus       dbViewerFocus
	explorer    DBExplorer
	queryPane   DBQueryPane
	resultsPane DBResultsPane
}

func NewDBViewerModal() DBViewerModal {
	return DBViewerModal{
		focus:       dbFocusSidebar,
		explorer:    newDBExplorer(),
		queryPane:   newDBQueryPane(),
		resultsPane: newDBResultsPane(),
	}
}

func (m *DBViewerModal) SetSize(w, h int) {
	m.width = w
	m.height = h

	// keep resultsPane scroll logic calibrated to its visible height
	var (
		innerH   = m.modalH() - 2
		bodyH    = innerH - 2 - 2 // titleRows=2, statusSep+statusBar=2
		queryH   = max(bodyH/3, 3)
		resultsH = bodyH - queryH - 1
	)

	m.resultsPane.setHeight(resultsH)
}

// setFocus transitions focus, blurring/focusing editor as needed.
func (m DBViewerModal) setFocus(f dbViewerFocus) (DBViewerModal, tea.Cmd) {
	if m.focus == f {
		return m, nil
	}

	if m.focus == dbFocusQuery {
		m.queryPane = m.queryPane.BlurEditor()
	}

	m.focus = f
	if f == dbFocusQuery {
		var cmd tea.Cmd
		m.queryPane, cmd = m.queryPane.FocusEditor()
		return m, cmd
	}

	return m, nil
}

func (m DBViewerModal) Update(msg tea.Msg) (DBViewerModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// forward non-key messages (cursor blink, etc.) to focused pane
		if m.focus == dbFocusQuery {
			var cmd tea.Cmd
			m.queryPane, cmd = m.queryPane.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	switch k.String() {
	case "esc", "q":
		return m, func() tea.Msg { return CancelOverlayMsg{} }

	case "tab":
		return m.setFocus((m.focus + 1) % 3)

	case "shift+tab":
		return m.setFocus((m.focus + 2) % 3)

	case "ctrl+l":
		return m.setFocus(dbFocusQuery)

	case "ctrl+j":
		if m.focus == dbFocusQuery {
			return m.setFocus(dbFocusResults)
		}

	case "ctrl+k":
		if m.focus == dbFocusResults {
			return m.setFocus(dbFocusQuery)
		}

	case "ctrl+h":
		if m.focus != dbFocusSidebar {
			return m.setFocus(dbFocusSidebar)
		}
		// already on sidebar — pass through so filter input handles it as backspace
		m.explorer, _ = m.explorer.Update(msg)

	default:
		switch m.focus {
		case dbFocusSidebar:
			m.explorer, _ = m.explorer.Update(msg)

		case dbFocusQuery:
			var cmd tea.Cmd
			m.queryPane, cmd = m.queryPane.Update(msg)
			return m, cmd

		case dbFocusResults:
			var cmd tea.Cmd
			m.resultsPane, cmd = m.resultsPane.Update(msg)
			return m, cmd
		}
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
	const sidebarW = 28
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

	// ── title styles ──
	iconS := lipgloss.NewStyle().Foreground(ColorAccent2).Background(ColorBg).PaddingLeft(1)
	labelS := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	dotS := lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg)
	dimS := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	faintS := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).PaddingRight(1)
	fgS := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)

	titleLeft := iconS.Render("▤") + " " +
		labelS.Render("Database Viewer") + "  " +
		dotS.Render("●") + " " +
		dimS.Render("simctl.db · SQLite 3.45") + "  " +
		faintS.Render("| simctl.db › main ›") + " " + fgS.Render("devices")

	titleRight := labelS.Render("Esc") + dimS.Render(" close  ") +
		labelS.Render("Ctrl+D") + dimS.Render(" toggle  ") +
		faintS.Render("[×]")

	// pad title row: left + spaces + right, total = innerW
	leftVis := lipgloss.Width(titleLeft)
	rightVis := lipgloss.Width(titleRight)
	gap := max(innerW-leftVis-rightVis, 1)
	titleRow := titleLeft + bgS.Render(strings.Repeat(" ", gap)) + titleRight

	var sb strings.Builder

	// Top border — ┬ is part of the outer border (accent colour), not the inner divider.
	sb.WriteString(outerS.Render("╭" + strings.Repeat("─", sidebarW) + "─" + strings.Repeat("─", rightW) + "╮"))
	sb.WriteByte('\n')

	// Title row (spans full innerW, ignores sidebar/right split).
	// gap already accounts for visible widths; titleRow is correctly sized.
	sb.WriteString(outerS.Render("│"))
	sb.WriteString(titleRow)
	sb.WriteString(outerS.Render("│"))
	sb.WriteByte('\n')

	// Title separator — dim horizontal rule across the full inner width.
	sb.WriteString(outerS.Render("│"))
	sb.WriteString(innerS.Render(strings.Repeat("─", sidebarW) + "┬" + strings.Repeat("─", rightW)))
	sb.WriteString(outerS.Render("│"))
	sb.WriteByte('\n')

	// Body rows — 2 rows consumed by title + separator; 2 by status sep + bar.
	const titleRows = 2
	bodyH := innerH - titleRows - 2

	// dividerRow is now relative to body start.
	bodyDividerRow := dividerRow

	resultsH := bodyH - bodyDividerRow - 1

	// Pre-render panels once for the full body height.
	sidebarRows := m.explorer.Rows(sidebarW, bodyH)
	queryPaneRows := m.queryPane.Rows(rightW, bodyDividerRow, m.focus == dbFocusQuery)
	resultsPaneRows := m.resultsPane.Rows(rightW, resultsH, m.focus == dbFocusResults)

	// Content rows.
	for row := range bodyH {
		sb.WriteString(outerS.Render("│"))
		if row < len(sidebarRows) {
			sb.WriteString(sidebarRows[row])
		} else {
			sb.WriteString(blank(sidebarW))
		}

		switch {
		case row == bodyDividerRow:
			sb.WriteString(innerS.Render("├" + strings.Repeat("─", rightW)))
			sb.WriteString(outerS.Render("│"))
		case row > bodyDividerRow:
			sb.WriteString(innerS.Render("│"))
			ri := row - bodyDividerRow - 1
			if ri < len(resultsPaneRows) {
				sb.WriteString(resultsPaneRows[ri])
			} else {
				sb.WriteString(blank(rightW))
			}
			sb.WriteString(outerS.Render("│"))
		default:
			sb.WriteString(innerS.Render("│"))
			if row < len(queryPaneRows) {
				sb.WriteString(queryPaneRows[row])
			} else {
				sb.WriteString(blank(rightW))
			}
			sb.WriteString(outerS.Render("│"))
		}

		sb.WriteByte('\n')
	}

	// Status separator — spans full inner width (sidebar + divW + right).
	sb.WriteString(outerS.Render("│"))
	sb.WriteString(innerS.Render(strings.Repeat("─", innerW)))
	sb.WriteString(outerS.Render("│"))
	sb.WriteByte('\n')

	// Status bar row.
	sb.WriteString(outerS.Render("│"))
	sb.WriteString(m.renderStatusBar(innerW))
	sb.WriteString(outerS.Render("│"))
	sb.WriteByte('\n')

	// Bottom border — ┴ is part of the outer border (accent color).
	sb.WriteString(outerS.Render("╰" + strings.Repeat("─", sidebarW) + "┴" + strings.Repeat("─", rightW) + "╯"))

	return sb.String()
}

func (m DBViewerModal) renderStatusBar(width int) string {
	bgS   := lipgloss.NewStyle().Background(ColorBg)
	dimS  := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	faintS := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	kS    := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	vS    := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	okS   := lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg)

	// mode badge: INSERT (warn bg) when query focused, NORMAL (accent bg) otherwise
	var modeBadge string
	if m.focus == dbFocusQuery {
		modeBadge = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorWarn).Bold(true).Padding(0, 1).
			Render("INSERT")
	} else {
		modeBadge = lipgloss.NewStyle().
			Foreground(ColorBg).Background(ColorAccent).Bold(true).Padding(0, 1).
			Render("NORMAL")
	}

	conn    := okS.Render("● connected to simctl.db")
	latency := dimS.Render("⟳ 14 ms · 21 rows · 9 cols")
	left    := modeBadge + bgS.Render("  ") + conn + bgS.Render("  ") + latency

	hints := strings.Join([]string{
		kS.Render("esc") + vS.Render(" normal"),
		kS.Render("F5") + vS.Render("/") + kS.Render("^enter") + vS.Render(" execute"),
		kS.Render("⇥") + vS.Render(" autocomplete"),
		kS.Render("^P") + vS.Render(" palette"),
		kS.Render("?") + vS.Render(" help"),
	}, faintS.Render("  "))

	gap := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
	line := left + bgS.Render(strings.Repeat(" ", gap)) + hints
	need := width - lipgloss.Width(line)
	if need > 0 {
		line += bgS.Render(strings.Repeat(" ", need))
	}
	return line
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
