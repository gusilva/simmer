package ui

import (
	"strings"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

// ── Apps tab ───────────────────────────────────────────────────────────

func (m MainPane) renderApps(w, h int) string {
	lines := make([]string, 0, h)

	// Filter bar — always rendered so layout height stays fixed.
	lines = append(lines, m.renderAppsFilterBar(w))

	listH := max(h-1, 1)

	if len(m.apps) == 0 {
		hint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("  no apps installed")
		if pad := w - lipgloss.Width(hint); pad > 0 {
			hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, hint)
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return strings.Join(lines, "\n")
	}

	filtered := m.filteredApps()

	if len(filtered) == 0 {
		hint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("  no matches")
		if pad := w - lipgloss.Width(hint); pad > 0 {
			hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, hint)
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return strings.Join(lines, "\n")
	}

	offset := 0
	if m.appsIdx >= listH {
		offset = m.appsIdx - listH + 1
	}
	end := min(offset+listH, len(filtered))
	for i := offset; i < end; i++ {
		streaming := m.selectedApp != nil && m.selectedApp.BundleID == filtered[i].BundleID
		lines = append(lines, m.renderAppRow(filtered[i], w, i == m.appsIdx, streaming))
	}

	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	return strings.Join(lines, "\n")
}

func (m MainPane) renderAppsFilterBar(w int) string {
	searchActive := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	searchDim := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	prefix := faint.Render(" /") + " "
	query := m.appsFilter
	cursor := ""
	if m.appsFiltering {
		cursor = searchActive.Render("█")
	}

	var text string
	if query == "" && !m.appsFiltering {
		text = faint.Render("filter apps…")
	} else {
		text = searchDim.Render(query) + cursor
	}

	row := prefix + text
	if pad := w - lipgloss.Width(row); pad > 0 {
		row += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
	}
	return row
}

func (m MainPane) renderAppRow(app device.App, w int, cursor bool, streaming bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	label := app.Label()
	meta := app.ShortVersion
	if meta == "" {
		meta = app.Version
	}

	// ⊙ = broadcast/signal indicator; 2 cells: space + glyph
	const streamIcon = " ⊙"
	const streamIconW = 2

	iconStr := ""
	iconW := 0
	if streaming {
		iconStr = streamIcon
		iconW = streamIconW
	}

	const (
		leadW  = 1
		trailW = 1
		minGap = 2
	)

	metaW := lipgloss.Width(meta)
	nameMax := max(w-leadW-iconW-metaW-trailW-minGap, 1)
	nameStr := truncateName(label, nameMax)
	gap := max(w-leadW-lipgloss.Width(nameStr)-iconW-metaW-trailW, minGap)

	if cursor {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		return sel.Render(" " + nameStr + iconStr + strings.Repeat(" ", gap) + meta + " ")
	}

	nameStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(nameStr)
	metaStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(meta)

	if streaming {
		iconStyled := lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Render(iconStr)
		return " " + nameStyled + iconStyled + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
	}

	return " " + nameStyled + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
}
