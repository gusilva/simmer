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
// If footer is non-empty it is always rendered as the last content line (sticky).
func RenderBox(title, badge, content, footer string, width, height int, focused bool) string {
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

	// Truncate title if it's too long for the box width.
	// Reserve at least 6 cells for borders and spacers: ╭──(3) + ──╮(3)
	maxTitleW := max(width-6, 0)
	displayTitle := truncateName(title, maxTitleW)
	titleText := titleStyle.Render(displayTitle)

	if badge != "" && lipgloss.Width(displayTitle)+2 <= maxTitleW {
		displayBadge := truncateName(badge, maxTitleW-lipgloss.Width(displayTitle)-1)
		titleText += " " + badgeStyle.Render(displayBadge)
	}
	titleW := lipgloss.Width(titleText)

	leftDash := 2
	// spaces: 1 before title, 1 after title
	// borders: 1 left, 1 right
	rightDash := max(width-2-leftDash-2-titleW, 0)

	top := border.Render("╭"+strings.Repeat("─", leftDash)+" ") +
		titleText +
		border.Render(" "+strings.Repeat("─", rightDash)+"╮")

	// Adjust for any rounding or min-width constraints to ensure exact width match
	actualW := lipgloss.Width(top)
	if actualW > width {
		// If overflowed, we need to reduce the title even further or remove dashes
		// For a 1-cell overflow, we can just remove the space after title if rightDash is 0
		if actualW == width+1 && rightDash == 0 {
			top = border.Render("╭"+strings.Repeat("─", leftDash)+" ") +
				titleText +
				border.Render("╮")
		}
	} else if actualW < width {
		// If underflowed, pad the right dashes
		top = border.Render("╭"+strings.Repeat("─", leftDash)+" ") +
			titleText +
			border.Render(" "+strings.Repeat("─", rightDash+(width-actualW))+"╮")
	}

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

	// Render the sticky footer line (always last inside the box).
	var footerLine string
	if footer != "" {
		fw := lipgloss.Width(footer)
		if fw > innerW {
			footer = lipgloss.NewStyle().MaxWidth(innerW).Render(footer)
			fw = lipgloss.Width(footer)
		}
		footerLine = border.Render("│") + footer + bg.Render(strings.Repeat(" ", innerW-fw)) + border.Render("│")
	}

	if height > 0 {
		innerH := max(height-2, 0)
		// Reserve one row for the footer when present.
		contentH := innerH
		if footer != "" {
			contentH = max(innerH-1, 0)
		}
		if len(lines) > contentH {
			lines = lines[:contentH]
		} else {
			blank := bg.Render(strings.Repeat(" ", innerW))
			fill := border.Render("│") + blank + border.Render("│")
			for len(lines) < contentH {
				lines = append(lines, fill)
			}
		}
	}

	if footerLine != "" {
		lines = append(lines, footerLine)
	}

	return strings.Join(append(append([]string{top}, lines...), bottom), "\n")
}
