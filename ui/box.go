package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderBox draws a bordered panel with a cutout title at the top-left.
// Each line of `content` is padded to the inner width with the panel background.
// If height > 0 the box is sized to exactly that many rows: extra content is
// truncated and shorter content is bottom-padded with bg-colored blank lines.
// If height <= 0 the box grows to fit its content.
func RenderBox(title, badge, content string, width, height int, focused bool) string {
	if width < 6 {
		return ""
	}

	borderC := ColorBorder
	titleC := ColorFgDim
	if focused {
		borderC = ColorBorderHi
		titleC = ColorBorderHi
	}

	border := lipgloss.NewStyle().Foreground(borderC).Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(titleC).Background(ColorBg)
	badgeStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	bg := lipgloss.NewStyle().Background(ColorBg)

	titleText := titleStyle.Render(title)
	if badge != "" {
		titleText += " " + badgeStyle.Render(badge)
	}
	titleW := lipgloss.Width(titleText)

	leftDash := 2
	rightDash := max(width-2-leftDash-2-titleW, 1)

	top := border.Render("╭"+strings.Repeat("─", leftDash)+" ") +
		titleText +
		border.Render(" "+strings.Repeat("─", rightDash)+"╮")

	bottom := border.Render("╰" + strings.Repeat("─", width-2) + "╯")

	innerW := width - 2
	var lines []string
	for line := range strings.SplitSeq(content, "\n") {
		w := lipgloss.Width(line)
		if w > innerW {
			line = lipgloss.NewStyle().MaxWidth(innerW).Render(line)
			w = lipgloss.Width(line)
		}
		padded := line + bg.Render(strings.Repeat(" ", innerW-w))
		lines = append(lines, border.Render("│")+padded+border.Render("│"))
	}

	if height > 0 {
		innerH := max(height-2, 0)
		if len(lines) > innerH {
			lines = lines[:innerH]
		} else {
			blank := bg.Render(strings.Repeat(" ", innerW))
			fill := border.Render("│") + blank + border.Render("│")
			for len(lines) < innerH {
				lines = append(lines, fill)
			}
		}
	}

	return strings.Join(append(append([]string{top}, lines...), bottom), "\n")
}
