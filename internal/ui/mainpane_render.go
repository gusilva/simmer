package ui

import (
	"fmt"
	"strings"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

// View renders the main pane: a persistent device header, then two stacked
// panels (Apps on top, Files below) of which one is expanded and the other is a
// one-line strip. Frame is composed manually so the Files divider rules can tee
// (┬/┴) into the vertical tree/preview separator.
func (m MainPane) View() string {
	if m.width < 12 || m.height < 5 {
		return ""
	}

	borderC := ColorBorder
	if m.focused {
		borderC = ColorBorderHi
	}
	frame := lipgloss.NewStyle().Foreground(borderC).Background(ColorBg)
	innerRule := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)

	innerW := m.width - 2
	innerH := m.height - 2

	treeW, _, _ := filesLayout(innerW)
	filesExpanded := m.panel == panelFiles && m.tree != nil

	rows := make([]string, 0, innerH)
	if m.active == nil {
		rows = append(rows, strings.Split(m.renderHint(innerW, innerH), "\n")...)
	} else {
		// Header: title, status/hint, rule.
		rows = append(rows, m.renderTitleRow(innerW))
		rows = append(rows, m.renderHeaderMeta(innerW))
		rows = append(rows, hrule(innerW, -1, "", innerRule))

		// Both regions always show their "[N] Label" line. Fixed chrome is the
		// 3 header rows + the two labels + the divider between the regions; the
		// expanded region fills whatever height is left.
		content := max(innerH-6, 1)

		// Apps region: "[2] Apps" label — carrying the filter input on its right
		// while expanded — then the list below.
		rows = append(rows, m.renderAppsLabel(innerW))
		if m.panel == panelApps {
			rows = append(rows, strings.Split(m.renderApps(innerW, content), "\n")...)
		}

		// Plain divider — the "[3] Files" label spans the full width, so the
		// tree/preview split only starts on the rows below it.
		rows = append(rows, hrule(innerW, -1, "", innerRule))

		// Files region: full-width label (path right-aligned), then tree +
		// preview below when expanded.
		rows = append(rows, m.renderPanelLabel(panelFiles, innerW, treeW, false))
		if m.panel == panelFiles {
			rows = append(rows, strings.Split(m.renderFiles(innerW, content), "\n")...)
		}
	}

	if len(rows) > innerH {
		rows = rows[:innerH]
	}
	for len(rows) < innerH {
		rows = append(rows, padBg(innerW))
	}

	side := frame.Render("│")
	wrapped := make([]string, 0, innerH+2)
	wrapped = append(wrapped, frame.Render("╭"+strings.Repeat("─", innerW)+"╮"))
	for _, r := range rows {
		wrapped = append(wrapped, side+r+side)
	}

	bottomDashes := strings.Repeat("─", innerW)
	if filesExpanded && treeW > 0 && treeW < innerW {
		bottomDashes = strings.Repeat("─", treeW) + "┴" + strings.Repeat("─", innerW-treeW-1)
	}
	wrapped = append(wrapped, frame.Render("╰"+bottomDashes+"╯"))

	return strings.Join(wrapped, "\n")
}

// HandleClick maps a click at (x, y) — local to this pane's own View()
// output (0,0 = the outer "╭" border) — to a panel switch and/or row
// selection. Row math mirrors View() exactly (same header/label/divider
// row counts, same scroll-window offset formula as renderApps/renderTreePane),
// so a click always lands on the row it visually looks like it hit.
func (m *MainPane) HandleClick(x, y int) {
	if m.active == nil || m.width < 12 || m.height < 5 {
		return
	}
	innerW := m.width - 2
	innerH := m.height - 2
	if x < 1 || x > innerW || y < 1 || y > innerH {
		return // border rows/columns
	}
	lx := x - 1 // 0-based column within the inner content
	ly := y - 1 // 0-based row within the inner content

	content := max(innerH-6, 1)
	treeW, _, _ := filesLayout(innerW)

	switch {
	case ly == 3: // "[2] Apps" label row
		m.panel = panelApps

	case m.panel == panelApps && ly >= 4 && ly < 4+content:
		m.panel = panelApps
		block := ly - 4
		listH := max(content-2, 1)
		if block < 1 || block-1 >= listH {
			return
		}
		offset := 0
		if m.appsIdx >= listH {
			offset = m.appsIdx - listH + 1
		}
		if idx := offset + block - 1; idx < len(m.filteredApps()) {
			m.appsIdx = idx
		}

	case (m.panel == panelApps && ly == 5+content) || (m.panel == panelFiles && ly == 5):
		m.panel = panelFiles

	case m.panel == panelFiles && ly >= 6 && ly < 6+content:
		m.panel = panelFiles
		block := ly - 6
		visibleH := max(content-1, 1)
		if block < 1 || block-1 >= visibleH || lx >= treeW {
			return
		}
		offset := 0
		if m.treeIdx >= visibleH {
			offset = m.treeIdx - visibleH + 1
		}
		if idx := offset + block - 1; idx < len(m.flattenTree()) {
			m.treeIdx = idx
		}
	}
}

// hrule renders a horizontal rule of `width` cells, optionally inserting a
// junction glyph at column `at`. Set at < 0 (or junction == "") for a plain rule.
func hrule(width, at int, junction string, style lipgloss.Style) string {
	if at < 0 || at >= width || junction == "" {
		return style.Render(strings.Repeat("─", width))
	}
	return style.Render(strings.Repeat("─", at) + junction + strings.Repeat("─", width-at-1))
}

// filesLayout returns the column split used by the expanded Files panel.
func filesLayout(innerW int) (treeW, sepW, previewW int) {
	sepW = 1
	treeW = max(innerW*52/100, 24)
	previewW = innerW - treeW - sepW
	if previewW < 16 {
		previewW = 16
		treeW = innerW - sepW - previewW
	}
	return
}

// ── rendering helpers ──────────────────────────────────────────────────

func (m MainPane) renderHint(innerW, innerH int) string {
	hint := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("  press space on a device to load")

	if pad := innerW - lipgloss.Width(hint); pad > 0 {
		hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
	}

	lines := []string{hint}
	for len(lines) < innerH {
		lines = append(lines, padBg(innerW))
	}
	return strings.Join(lines, "\n")
}

func (m MainPane) renderTitleRow(innerW int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	dotC := ColorFgFaint
	if m.active.Status == device.StatusRunning {
		dotC = ColorOk
	}
	dot := lipgloss.NewStyle().Foreground(dotC).Background(ColorBg).Render("●")
	name := lipgloss.NewStyle().
		Foreground(ColorBorderHi).
		Background(ColorBg).
		Bold(true).
		Render(m.active.Name)
	sep := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("·")
	osText := fmt.Sprintf("%s %s", m.active.Platform, m.active.Version)
	osLabel := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render(osText)

	udidShort := m.active.ID
	if len(udidShort) > 8 {
		udidShort = udidShort[:8] + "…"
	}
	udid := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("udid " + udidShort)

	left := " " + dot + " " + name + " " + sep + " " + osLabel
	gap := max(innerW-lipgloss.Width(left)-lipgloss.Width(udid)-1, 1)
	return left + bg.Render(strings.Repeat(" ", gap)) + udid + bg.Render(" ")
}

// renderHeaderMeta is the second header line: live device status plus the
// "[i] more info" affordance (hidden while the info overlay is open).
func (m MainPane) renderHeaderMeta(innerW int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	statusC := ColorFgFaint
	if m.active.Status == device.StatusRunning {
		statusC = ColorOk
	}
	label := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("status ")
	val := lipgloss.NewStyle().Foreground(statusC).Background(ColorBg).Render(string(m.active.Status))

	row := " " + label + val
	if !m.infoOpen {
		hint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("[i] more info")
		row += bg.Render("   ") + hint
	}
	if pad := innerW - lipgloss.Width(row); pad > 0 {
		row += bg.Render(strings.Repeat(" ", pad))
	}
	return row
}

// renderPanelLabel draws a region's "[N] Label" line. It is shown for both
// regions at all times — alone for the collapsed region, as a heading above the
// content for the expanded one. For the Files region the current tree path
// rides on the right of the same line ("[3] Files        …/Documents"). When
// withSep is true (expanded Files) the line carries the tree/preview separator
// at column treeW so the ┬/┴ tees align with the rows below.
func (m MainPane) renderPanelLabel(p mainPanel, innerW, treeW int, withSep bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)
	numStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	nameStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)
	pathStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	num, label := "[2] ", "Apps"
	if p == panelFiles {
		num, label = "[3] ", "Files"
	}
	text := " " + numStyle.Render(num) + nameStyle.Render(label)
	textW := 1 + lipgloss.Width(num) + lipgloss.Width(label)

	// leftW is the width of the "[N] Label + path" block; for Files it fills the
	// tree column (treeW) when expanded, else the whole line.
	leftW := innerW
	if withSep && treeW > 0 && treeW < innerW {
		leftW = treeW
	}

	left := text
	if p == panelFiles && m.tree != nil && m.tree.Path != "" {
		avail := max(leftW-textW-2, 0)
		path := truncPathLeft(m.tree.Path, avail)
		if lipgloss.Width(path) > 0 {
			gap := max(leftW-textW-lipgloss.Width(path)-1, 1)
			left = text + bg.Render(strings.Repeat(" ", gap)) + pathStyle.Render(path) + bg.Render(" ")
		}
	}
	leftFilled := left
	if pad := leftW - lipgloss.Width(left); pad > 0 {
		leftFilled = left + bg.Render(strings.Repeat(" ", pad))
	}

	if withSep && treeW > 0 && treeW < innerW {
		right := innerW - treeW - 1
		return leftFilled + sepStyle.Render("│") + bg.Render(strings.Repeat(" ", max(right, 0)))
	}
	return leftFilled
}

// truncPathLeft trims a path from the left, keeping the tail behind a leading …
func truncPathLeft(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if lipgloss.Width(string(runes[i:])) <= max-1 {
			return "…" + string(runes[i:])
		}
	}
	return "…"
}

// renderAppsLabel draws the "[2] Apps" line. While the Apps panel is expanded
// the filter input rides on the right of the same line:
// "[2] Apps            / filter apps…".
func (m MainPane) renderAppsLabel(innerW int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)
	numStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	nameStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)

	left := " " + numStyle.Render("[2] ") + nameStyle.Render("Apps")
	leftW := 1 + 4 + 4

	if m.panel != panelApps {
		if pad := innerW - leftW; pad > 0 {
			left += bg.Render(strings.Repeat(" ", pad))
		}
		return left
	}

	filter := m.renderAppsFilterBar()
	fw := lipgloss.Width(filter)
	gap := max(innerW-leftW-fw-1, 1)
	return left + bg.Render(strings.Repeat(" ", gap)) + filter + bg.Render(" ")
}

// ── small utilities ────────────────────────────────────────────────────

func padBg(n int) string {
	if n <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", n))
}
