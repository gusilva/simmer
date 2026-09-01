package ui

import (
	"fmt"
	"strings"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

// View renders the main pane with a manually composed frame so the inner
// divider rules can tee (┬/┴) into the vertical Files separator.
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

	rows := make([]string, 0, innerH)
	if m.active == nil {
		rows = append(rows, strings.Split(m.renderHint(innerW, innerH), "\n")...)
	} else {
		rows = append(rows, m.renderTitleRow(innerW))
		rows = append(rows, hrule(innerW, -1, "", innerRule))
		rows = append(rows, RenderTabs(mainTabs, int(m.tab), innerW))
		junction := -1
		if m.tab == TabFiles && m.tree != nil {
			junction = treeW
		}
		rows = append(rows, hrule(innerW, junction, "┬", innerRule))

		contentH := max(innerH-len(rows), 1)
		rows = append(rows, strings.Split(m.renderTabContent(innerW, contentH), "\n")...)
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
	if m.active != nil && m.tab == TabFiles && m.tree != nil && treeW > 0 && treeW < innerW {
		bottomDashes = strings.Repeat("─", treeW) + "┴" + strings.Repeat("─", innerW-treeW-1)
	}
	wrapped = append(wrapped, frame.Render("╰"+bottomDashes+"╯"))

	return strings.Join(wrapped, "\n")
}

// hrule renders a horizontal rule of `width` cells, optionally inserting a
// junction glyph at column `at`. Set at < 0 (or junction == "") for a plain rule.
func hrule(width, at int, junction string, style lipgloss.Style) string {
	if at < 0 || at >= width || junction == "" {
		return style.Render(strings.Repeat("─", width))
	}
	return style.Render(strings.Repeat("─", at) + junction + strings.Repeat("─", width-at-1))
}

// filesLayout returns the column split used by the Files tab.
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

func (m MainPane) renderTabContent(innerW, innerH int) string {
	switch m.tab {
	case TabFiles:
		return m.renderFiles(innerW, innerH)
	case TabApps:
		return m.renderApps(innerW, innerH)
	case TabInfo:
		return m.renderInfo(innerW, innerH)
	default:
		return m.renderPlaceholder(innerW, innerH)
	}
}

func (m MainPane) renderPlaceholder(innerW, innerH int) string {
	body := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("  (not implemented yet)")
	lines := []string{body}
	for len(lines) < innerH {
		lines = append(lines, padBg(innerW))
	}
	return strings.Join(lines, "\n")
}

// ── small utilities ────────────────────────────────────────────────────

func padBg(n int) string {
	if n <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", n))
}
