package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderBox draws a bordered panel with a cutout title at the top-left.
// Each line of `content` is padded to the inner width with the panel background.
// If height > 0 the box is sized to exactly that many rows: extra content is
// truncated and shorter content is bottom-padded with bg-colored blank lines.
// If height <= 0 the box grows to fit its content.
// If footer is non-empty it is always rendered as the last content line (sticky).
// keyHint, if non-empty, renders as ╭─[KEY]─ title before the title text.
// bottomBadge, if non-empty, is right-aligned in the bottom border.
func RenderBox(title, badge, content, footer string, width, height int, focused bool, keyHint, bottomBadge string) string {
	if width < 6 {
		return ""
	}

	borderC := ColorBorder
	titleC := ColorFgDim
	if focused {
		borderC = ColorBorderHi
		titleC = ColorBorderHi
	}

	titleStyle := lipgloss.NewStyle().Foreground(titleC).Background(ColorBg)
	badgeStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	leftPartW := leftBorderWidth(keyHint)
	const rightOverhead = 2 // space after title + ╮
	maxTitleW := max(width-leftPartW-rightOverhead, 0)

	displayTitle := truncateName(title, maxTitleW)
	titleText := titleStyle.Render(displayTitle)

	if badge != "" && lipgloss.Width(displayTitle)+2 <= maxTitleW {
		displayBadge := truncateName(badge, maxTitleW-lipgloss.Width(displayTitle)-1)
		titleText += " " + badgeStyle.Render(displayBadge)
	}

	return renderBoxCore(titleText, content, footer, width, height, borderC, keyHint, bottomBadge)
}

// renderBoxRaw is like RenderBox but accepts a pre-styled title and does not
// apply any additional colour to it. The border colour still reflects focused.
func renderBoxRaw(title, content, footer string, width, height int, focused bool, keyHint, bottomBadge string) string {
	if width < 6 {
		return ""
	}
	borderC := ColorBorder
	if focused {
		borderC = ColorBorderHi
	}
	return renderBoxCore(title, content, footer, width, height, borderC, keyHint, bottomBadge)
}

// renderBoxCore is the shared implementation. styledTitle is used verbatim.
func renderBoxCore(styledTitle, content, footer string, width, height int, borderC color.Color, keyHint, bottomBadge string) string {
	border := lipgloss.NewStyle().Foreground(borderC).Background(ColorBg)
	bg := lipgloss.NewStyle().Background(ColorBg)

	titleW := lipgloss.Width(styledTitle)
	leftPartW := leftBorderWidth(keyHint)
	const rightOverhead = 2
	rightDash := max(width-leftPartW-titleW-rightOverhead, 0)

	var leftPart string
	if keyHint != "" {
		leftPart = border.Render("╭─[" + keyHint + "]─ ")
	} else {
		leftPart = border.Render("╭── ")
	}

	top := leftPart + styledTitle + border.Render(" "+strings.Repeat("─", rightDash)+"╮")

	actualW := lipgloss.Width(top)
	if actualW > width {
		if actualW == width+1 && rightDash == 0 {
			top = leftPart + styledTitle + border.Render("╮")
		}
	} else if actualW < width {
		top = leftPart + styledTitle + border.Render(" "+strings.Repeat("─", rightDash+(width-actualW))+"╮")
	}

	// Bottom border with optional right-aligned badge.
	inner := width - 2
	var bottom string
	if bottomBadge != "" {
		badgeW := lipgloss.Width(bottomBadge)
		const rightTrail = 2
		leftDashes := inner - badgeW - rightTrail
		if leftDashes >= 0 {
			badgeSt := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
			bottom = border.Render("╰"+strings.Repeat("─", leftDashes)) +
				badgeSt.Render(bottomBadge) +
				border.Render(strings.Repeat("─", rightTrail)+"╯")
		} else {
			bottom = border.Render("╰" + strings.Repeat("─", inner) + "╯")
		}
	} else {
		bottom = border.Render("╰" + strings.Repeat("─", inner) + "╯")
	}

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

// leftBorderWidth returns the visible cell width of the left border segment
// (from ╭ up to and including the space before the title).
func leftBorderWidth(keyHint string) int {
	if keyHint == "" {
		return 4 // ╭──<space>
	}
	// ╭─[KEY]─<space>
	return 1 + 1 + 1 + len(keyHint) + 1 + 1 + 1
}
