package theme

import "charm.land/lipgloss/v2"

// LiveBadge renders the Active-instance indicator: whether this Simmer is
// the one recording the View state, and how to become it when it is not.
// The TUI top bar and the DB viewer title (which covers the top bar) share
// it so the two can never disagree.
func LiveBadge(live bool) string {
	if live {
		return lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Render("● live")
	}
	return lipgloss.NewStyle().Foreground(ColorWarn).Background(ColorBg).Render("○ not live · ^T take over")
}
