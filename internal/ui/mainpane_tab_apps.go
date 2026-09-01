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
	local := m.active != nil && m.active.Kind == device.KindVirtual
	for i := offset; i < end; i++ {
		logging := m.loggingBundle != "" && m.loggingBundle == filtered[i].BundleID
		lines = append(lines, m.renderAppRow(filtered[i], w, i == m.appsIdx, logging, local && filtered[i].IsLocal()))
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

func (m MainPane) renderAppRow(app device.App, w int, cursor bool, logging bool, local bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	label := app.Label()
	meta := app.ShortVersion
	if meta == "" {
		meta = app.Version
	}

	// [dev] = locally-built badge; 6 cells: space + [dev]
	const devBadge = " [dev]"
	const devBadgeW = 6
	// [logging] = logs being written to file; 10 cells: space + [logging]
	const logBadge = " [logging]"
	const logBadgeW = 10
	// ⊙ = app pinned (via space) for Files-tab browsing; 2 cells: space + glyph
	const pinIcon = " ⊙"
	const pinIconW = 2

	pinned := m.selectedApp != nil && m.selectedApp.BundleID == app.BundleID

	devStr := ""
	devW := 0
	if local {
		devStr = devBadge
		devW = devBadgeW
	}
	logStr := ""
	logW := 0
	if logging {
		logStr = logBadge
		logW = logBadgeW
	}
	pinStr := ""
	pinW := 0
	if pinned {
		pinStr = pinIcon
		pinW = pinIconW
	}
	iconW := devW + logW + pinW

	const (
		leadW  = 1
		trailW = 1
		minGap = 2
	)

	metaW := lipgloss.Width(meta)
	nameMax := max(w-leadW-iconW-metaW-trailW-minGap, 1)
	nameStr := truncateName(label, nameMax)
	gap := max(w-leadW-lipgloss.Width(nameStr)-iconW-metaW-trailW, minGap)

	system := app.Type == "System"

	if cursor {
		selBg := ColorAccent
		if system {
			selBg = ColorFgDim
		}
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(selBg).Bold(true)
		return sel.Render(" " + nameStr + devStr + logStr + pinStr + strings.Repeat(" ", gap) + meta + " ")
	}

	nameFg := ColorFg
	if system {
		nameFg = ColorFgDim
	}
	nameStyled := lipgloss.NewStyle().Foreground(nameFg).Background(ColorBg).Render(nameStr)
	devStyled := lipgloss.NewStyle().Foreground(ColorOrange).Background(ColorBg).Render(devStr)
	logStyled := lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg).Bold(true).Render(logStr)
	pinStyled := lipgloss.NewStyle().Foreground(ColorAccent2).Background(ColorBg).Render(pinStr)
	metaStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(meta)

	return " " + nameStyled + devStyled + logStyled + pinStyled + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
}
