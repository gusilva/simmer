package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// View renders the DB viewer as an ANSI string for overlay placement.
func (m DBViewerModal) View() string {
	l := computeDBViewerLayout(m.width, m.height)
	s := m.styles

	var sb strings.Builder
	m.renderTopBorder(&sb, l, s)
	m.renderTitleRow(&sb, l, s)
	m.renderTitleSep(&sb, l, s)
	m.renderBody(&sb, l, s)
	m.renderStatusSep(&sb, l, s)
	m.renderStatusRow(&sb, l, s)
	m.renderBottomBorder(&sb, l, s)
	return sb.String()
}

func (m DBViewerModal) renderTopBorder(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	sb.WriteString(s.Outer.Render("╭" + strings.Repeat("─", l.SidebarW) + "─" + strings.Repeat("─", l.RightW) + "╮"))
	sb.WriteByte('\n')
}

func (m DBViewerModal) renderTitleRow(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	blank := func(n int) string { return s.Bg.Render(strings.Repeat(" ", n)) }

	left := s.Icon.Render("▤") + " " +
		s.Label.Render("Database Viewer") + "  " +
		s.Dot.Render("●") + " " +
		s.Dim.Render("simctl.db · SQLite 3.45") + "  " +
		s.Faint.Render("| simctl.db › main ›") + " " + s.Fg.Render("devices")

	right := s.Label.Render("Esc") + s.Dim.Render(" close  ") +
		s.Label.Render("Ctrl+D") + s.Dim.Render(" toggle  ") +
		s.Faint.Render("[×]")

	gap := max(l.InnerW-lipgloss.Width(left)-lipgloss.Width(right), 1)
	titleRow := left + blank(gap) + right

	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(titleRow)
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m DBViewerModal) renderTitleSep(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(s.Inner.Render(strings.Repeat("─", l.SidebarW) + "┬" + strings.Repeat("─", l.RightW)))
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m DBViewerModal) renderBody(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	blank := func(n int) string { return s.Bg.Render(strings.Repeat(" ", n)) }

	sidebarRows := m.panes[dbPaneSidebar].Rows(l.SidebarW, l.BodyH, m.focus == dbPaneSidebar)
	queryRows   := m.panes[dbPaneQuery].Rows(l.RightW, l.DivRow, m.focus == dbPaneQuery)
	resultsRows := m.panes[dbPaneResults].Rows(l.RightW, l.ResultsH, m.focus == dbPaneResults)

	for row := range l.BodyH {
		sb.WriteString(s.Outer.Render("│"))

		if row < len(sidebarRows) {
			sb.WriteString(sidebarRows[row])
		} else {
			sb.WriteString(blank(l.SidebarW))
		}

		switch {
		case row == l.DivRow:
			sb.WriteString(s.Inner.Render("├" + strings.Repeat("─", l.RightW)))
			sb.WriteString(s.Outer.Render("│"))
		case row > l.DivRow:
			sb.WriteString(s.Inner.Render("│"))
			ri := row - l.DivRow - 1
			if ri < len(resultsRows) {
				sb.WriteString(resultsRows[ri])
			} else {
				sb.WriteString(blank(l.RightW))
			}
			sb.WriteString(s.Outer.Render("│"))
		default:
			sb.WriteString(s.Inner.Render("│"))
			if row < len(queryRows) {
				sb.WriteString(queryRows[row])
			} else {
				sb.WriteString(blank(l.RightW))
			}
			sb.WriteString(s.Outer.Render("│"))
		}
		sb.WriteByte('\n')
	}
}

func (m DBViewerModal) renderStatusSep(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(s.Inner.Render(strings.Repeat("─", l.InnerW)))
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m DBViewerModal) renderStatusRow(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	var modeBadge string
	if m.focus == dbPaneQuery {
		modeBadge = s.ModeBadgeInsert.Render("INSERT")
	} else {
		modeBadge = s.ModeBadgeNormal.Render("NORMAL")
	}

	conn    := s.Ok.Render("● connected to simctl.db")
	latency := s.Dim.Render("⟳ 14 ms · 21 rows · 9 cols")
	left    := modeBadge + s.Bg.Render("  ") + conn + s.Bg.Render("  ") + latency

	hints := strings.Join([]string{
		s.Key.Render("esc") + s.Val.Render(" normal"),
		s.Key.Render("F5") + s.Val.Render("/") + s.Key.Render("^enter") + s.Val.Render(" execute"),
		s.Key.Render("⇥") + s.Val.Render(" autocomplete"),
		s.Key.Render("^P") + s.Val.Render(" palette"),
		s.Key.Render("?") + s.Val.Render(" help"),
	}, s.Faint.Render("  "))

	width := l.InnerW
	gap   := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
	line  := left + s.Bg.Render(strings.Repeat(" ", gap)) + hints
	if need := width - lipgloss.Width(line); need > 0 {
		line += s.Bg.Render(strings.Repeat(" ", need))
	}

	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(line)
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m DBViewerModal) renderBottomBorder(sb *strings.Builder, l dbViewerLayout, s dbViewerStyles) {
	sb.WriteString(s.Outer.Render("╰" + strings.Repeat("─", l.SidebarW) + "┴" + strings.Repeat("─", l.RightW) + "╯"))
}
