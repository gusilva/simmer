package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// ── Logs tab ───────────────────────────────────────────────────────────

func (m MainPane) renderLogs(w, h int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	if m.logBundle == "" {
		rendered := lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("  no selected app — press space on an app to start streaming")
		if pad := w - lipgloss.Width(rendered); pad > 0 {
			rendered += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines := []string{rendered}
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return strings.Join(lines, "\n")
	}

	headerL := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("  streaming ")
	headerN := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(m.logBundle)
	header := headerL + headerN
	rule := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Background(ColorBg).
		Render(strings.Repeat("─", w))

	lines := []string{header, rule}

	for vpLine := range strings.SplitSeq(m.logsVP.View(), "\n") {
		lines = append(lines, " "+vpLine)
	}

	for i, line := range lines {
		if pad := w - lipgloss.Width(line); pad > 0 {
			lines[i] = line + bg.Render(strings.Repeat(" ", pad))
		}
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	return strings.Join(lines, "\n")
}
