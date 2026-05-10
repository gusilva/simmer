package ui

import (
	"strings"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

// ── Info tab ───────────────────────────────────────────────────────────

func (m MainPane) renderInfo(w, h int) string {

	if len(m.info.Fields) == 0 {
		hint := lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("  loading info…")

		if pad := w - lipgloss.Width(hint); pad > 0 {
			hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}

		lines := []string{hint}
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}

		return strings.Join(lines, "\n")
	}

	visibleH := max(h, 1)
	offset := 0
	if m.infoIdx >= visibleH {
		offset = m.infoIdx - visibleH + 1
	}
	end := min(offset+visibleH, len(m.info.Fields))

	keyW := infoKeyWidth(m.info.Fields)

	lines := make([]string, 0, h)
	for i := offset; i < end; i++ {
		f := m.info.Fields[i]
		lines = append(lines, renderInfoRow(f.Key, f.Value, keyW, w, i == m.infoIdx))
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	return strings.Join(lines, "\n")
}

// infoKeyWidth picks the column width for keys: longest visible key, capped at
// 14 so a single long label doesn't squeeze the value column.
func infoKeyWidth(fields []device.InfoField) int {
	const cap_ = 14
	w := 0
	for _, f := range fields {
		if kw := lipgloss.Width(f.Key); kw > w {
			w = kw
		}
	}
	if w > cap_ {
		w = cap_
	}
	if w < 4 {
		w = 4
	}
	return w
}

func renderInfoRow(k, v string, keyW, w int, selected bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	keyText := k
	if lipgloss.Width(keyText) > keyW {
		keyText = truncateName(keyText, keyW)
	}
	keyText = padRight(keyText, keyW)

	const leadW = 1
	const sepW = 1
	const trailW = 1
	valBudget := max(w-leadW-keyW-sepW-trailW, 1)
	if lipgloss.Width(v) > valBudget {
		v = truncateName(v, valBudget)
	}

	if selected {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		return sel.Render(" " + keyText + " " + v + strings.Repeat(" ", valBudget-lipgloss.Width(v)) + " ")
	}

	keyStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(keyText)
	valStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(v)
	row := " " + keyStyled + " " + valStyled
	if pad := w - lipgloss.Width(row) - trailW; pad > 0 {
		row += bg.Render(strings.Repeat(" ", pad))
	}
	return row + bg.Render(" ")
}
