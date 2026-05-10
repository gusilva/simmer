package dbviewer

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// View renders the DB viewer as an ANSI string for overlay placement.
func (m Modal) View() string {
	l := computeLayout(m.width, m.height)
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

func (m Modal) renderTopBorder(sb *strings.Builder, l layout, s viewerStyles) {
	sb.WriteString(s.Outer.Render("╭" + strings.Repeat("─", l.SidebarW) + "─" + strings.Repeat("─", l.RightW) + "╮"))
	sb.WriteByte('\n')
}

func (m Modal) renderTitleRow(sb *strings.Builder, l layout, s viewerStyles) {
	blank := func(n int) string { return s.Bg.Render(strings.Repeat(" ", n)) }

	dbLabel := "database"
	if m.dbName != "" {
		dbLabel = m.dbName
	}

	verLabel := "SQLite"
	if m.dbName != "" && m.sqliteVersion == "" {
		verLabel = "SQLite …"
	} else if m.sqliteVersion != "" {
		verLabel = "SQLite " + m.sqliteVersion
	}

	right := s.Label.Render("Esc") + s.Dim.Render(" close  ") +
		s.Label.Render("Ctrl+D") + s.Dim.Render(" toggle  ") +
		s.Faint.Render("[×]")

	prefix := s.Icon.Render("▤") + " " +
		s.Label.Render("Database Viewer") + "  " +
		s.Dot.Render("●") + " " +
		s.Dim.Render(dbLabel+" · "+verLabel) + "  " +
		s.Faint.Render("| ")

	const minGap = 1
	breadcrumbBudget := max(l.InnerW-lipgloss.Width(prefix)-lipgloss.Width(right)-minGap, 8)

	left := prefix + m.renderPathBreadcrumb(s, breadcrumbBudget)

	gap := max(l.InnerW-lipgloss.Width(left)-lipgloss.Width(right), 1)
	titleRow := left + blank(gap) + right

	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(titleRow)
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m Modal) renderTitleSep(sb *strings.Builder, l layout, s viewerStyles) {
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(s.Inner.Render(strings.Repeat("─", l.SidebarW) + "┬" + strings.Repeat("─", l.RightW)))
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m Modal) renderBody(sb *strings.Builder, l layout, s viewerStyles) {
	if m.settingsOpen {
		m.renderSettingsBody(sb, l, s)
		return
	}
	blank := func(n int) string { return s.Bg.Render(strings.Repeat(" ", n)) }

	sidebarRows := m.panes[paneSidebar].Rows(l.SidebarW, l.BodyH, m.focus == paneSidebar)
	queryRows := m.panes[paneQuery].Rows(l.RightW, l.DivRow, m.focus == paneQuery)
	resultsRows := m.panes[paneResults].Rows(l.RightW, l.ResultsH, m.focus == paneResults)

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

func (m Modal) renderSettingsBody(sb *strings.Builder, l layout, s viewerStyles) {
	rows := m.settings.Rows(l.InnerW, l.BodyH)
	for _, row := range rows {
		sb.WriteString(s.Outer.Render("│"))
		sb.WriteString(row)
		sb.WriteString(s.Outer.Render("│"))
		sb.WriteByte('\n')
	}
}

func (m Modal) renderStatusSep(sb *strings.Builder, l layout, s viewerStyles) {
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(s.Inner.Render(strings.Repeat("─", l.InnerW)))
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

func (m Modal) renderStatusRow(sb *strings.Builder, l layout, s viewerStyles) {
	var modeBadge string
	if m.focus == paneQuery {
		modeBadge = s.ModeBadgeInsert.Render("INSERT")
	} else {
		modeBadge = s.ModeBadgeNormal.Render("NORMAL")
	}

	// Transient status message (errors, save confirmation) takes priority over query stats.
	var statsOrMsg string
	if m.statusMsg != "" {
		if m.statusIsErr {
			statsOrMsg = s.Err.Render("✕ " + m.statusMsg)
		} else {
			statsOrMsg = s.Ok.Render("✓ " + m.statusMsg)
		}
	} else if m.lastStats.hasData {
		statsOrMsg = s.Dim.Render(fmt.Sprintf("⟳ %s  %d rows  %d cols",
			formatDuration(m.lastStats.elapsed),
			m.lastStats.rowCount,
			m.lastStats.colCount,
		))
	}

	left := modeBadge + s.Bg.Render("  ")
	if statsOrMsg != "" {
		left += s.Bg.Render("  ") + statsOrMsg
	}

	hints := strings.Join([]string{
		s.Key.Render("F5") + s.Val.Render(" execute"),
		s.Key.Render("⌘S") + s.Val.Render(" save"),
		s.Key.Render("^S") + s.Val.Render(" settings"),
		s.Key.Render("?") + s.Val.Render(" help"),
	}, s.Faint.Render("  "))

	width := l.InnerW
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
	line := left + s.Bg.Render(strings.Repeat(" ", gap)) + hints
	if need := width - lipgloss.Width(line); need > 0 {
		line += s.Bg.Render(strings.Repeat(" ", need))
	}

	sb.WriteString(s.Outer.Render("│"))
	sb.WriteString(line)
	sb.WriteString(s.Outer.Render("│"))
	sb.WriteByte('\n')
}

// formatDuration renders a duration as a compact human string.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dμs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
}

func (m Modal) renderBottomBorder(sb *strings.Builder, l layout, s viewerStyles) {
	sb.WriteString(s.Outer.Render("╰" + strings.Repeat("─", l.SidebarW) + "─" + strings.Repeat("─", l.RightW) + "╯"))
}

// [TODO] The breadcrumb rendering logic is a bit complex. Is it a useful feat?
// renderPathBreadcrumb splits m.dbPath into "/" segments and renders them as
// "seg1 › seg2 › … › last" where the final segment uses the foreground style
// and parent segments use the faint style. If the full breadcrumb exceeds
// budget (visible characters), middle segments are replaced with "…" until it
// fits: "first › … › last", then "… › last", then a truncated last segment.
func (m Modal) renderPathBreadcrumb(s viewerStyles, budget int) string {
	if m.dbPath == "" {
		return s.Fg.Render("database")
	}

	var parts []string
	for p := range strings.SplitSeq(m.dbPath, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return s.Fg.Render(m.dbPath)
	}

	plainSep := " › "
	ellipsis := "…"

	// joinPlain returns the plain (unstyled) width of rendering the given segments.
	// positions: 0=first kept from head, rest from tail.
	renderBreadcrumb := func(head, tail []string, withEllipsis bool) string {
		var sb strings.Builder
		all := append([]string(nil), head...)
		if withEllipsis {
			all = append(all, ellipsis)
		}
		all = append(all, tail...)
		for i, p := range all {
			if i > 0 {
				sb.WriteString(s.Faint.Render(plainSep))
			}
			isLast := i == len(all)-1
			isEllipsis := withEllipsis && i == len(head)
			if isLast {
				sb.WriteString(s.Fg.Render(p))
			} else if isEllipsis {
				sb.WriteString(s.Faint.Render(p))
			} else {
				sb.WriteString(s.Faint.Render(p))
			}
		}
		return sb.String()
	}

	plainWidth := func(head, tail []string, withEllipsis bool) int {
		all := append([]string(nil), head...)
		if withEllipsis {
			all = append(all, ellipsis)
		}
		all = append(all, tail...)
		w := 0
		for i, p := range all {
			if i > 0 {
				w += len(plainSep)
			}
			w += len([]rune(p))
		}
		return w
	}

	last := parts[len(parts)-1]
	head := parts[:len(parts)-1]

	// Try full path first.
	if plainWidth(head, []string{last}, false) <= budget {
		return renderBreadcrumb(head, []string{last}, false)
	}

	// Drop middle segments one by one from the right of head, keeping first.
	for len(head) > 1 {
		head = head[:len(head)-1]
		if plainWidth(head[:1], []string{last}, true) <= budget {
			return renderBreadcrumb(head[:1], []string{last}, true)
		}
	}

	// No head at all: "… › last".
	if plainWidth(nil, []string{last}, true) <= budget {
		return renderBreadcrumb(nil, []string{last}, true)
	}

	// Last segment itself is too long; truncate it.
	runes := []rune(last)
	for len(runes) > 1 && len([]rune(ellipsis))+len(runes)+len([]rune(plainSep)) > budget {
		runes = runes[:len(runes)-1]
	}
	truncated := string(runes) + ellipsis
	return s.Fg.Render(truncated)
}
