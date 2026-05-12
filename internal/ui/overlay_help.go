package ui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/lipgloss/v2"
)

// HelpOverlay renders context-specific key bindings as a floating panel.
type HelpOverlay struct {
	model help.Model
	keys  help.KeyMap
	title string
}

// NewHelpOverlay returns an overlay showing the given key map with matching title.
func NewHelpOverlay(title string, keys help.KeyMap) HelpOverlay {
	h := NewHelpModel()
	h.ShowAll = true
	return HelpOverlay{model: h, keys: keys, title: title}
}

// View renders the overlay box as an ANSI string for compositor placement.
// Content width is derived from the unconstrained rendered output so no line
// is ever word-wrapped or truncated by lipgloss.
func (o HelpOverlay) View() string {
	// Render at unconstrained width so the help component never truncates.
	h := o.model
	h.SetWidth(0)
	body := h.View(o.keys)

	titleStr := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Background(ColorBg).
		Bold(true).
		Render(o.title)

	dismiss := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("? / esc  close")

	// Measure the widest visible line across all content sections.
	contentW := 0
	for _, line := range strings.Split(body, "\n") {
		if w := lipgloss.Width(line); w > contentW {
			contentW = w
		}
	}
	if w := lipgloss.Width(titleStr); w > contentW {
		contentW = w
	}
	if w := lipgloss.Width(dismiss); w > contentW {
		contentW = w
	}

	// Manually pad every line of the inner block to contentW so that lipgloss
	// does not need to apply its own Width (which would word-wrap the ANSI text).
	inner := titleStr + "\n\n" + body + "\n\n" + dismiss
	var paddedLines []string
	bg := lipgloss.NewStyle().Background(ColorBg)
	for line := range strings.SplitSeq(inner, "\n") {
		if pad := contentW - lipgloss.Width(line); pad > 0 {
			line += bg.Render(strings.Repeat(" ", pad))
		}
		paddedLines = append(paddedLines, line)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	// Render without Width so lipgloss never word-wraps the pre-formatted text.
	return box.Render(strings.Join(paddedLines, "\n"))
}
