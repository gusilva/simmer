package dbviewer

import (
	"image/color"

	"simmer/internal/theme"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	goeditor "github.com/gusilva/goeditor"
)

// sqlLexer is the chroma lexer the query editor highlights with. "sqlite3"
// names no lexer and would silently fall back to plain text.
const sqlLexer = "sql"

// sqlStyle is the chroma style registered from the Simmer palette, so the
// editor's syntax colours match the rest of the TUI.
const sqlStyle = "simmer"

func init() {
	hex := func(c color.Color) string {
		r, g, b, _ := c.RGBA()
		return "#" + hex2(r>>8) + hex2(g>>8) + hex2(b>>8)
	}
	styles.Register(chroma.MustNewStyle(sqlStyle, chroma.StyleEntries{
		chroma.Text:          hex(theme.ColorFg),
		chroma.Keyword:       "bold " + hex(theme.ColorAccent),
		chroma.NameBuiltin:   hex(theme.ColorAccent2),
		chroma.NameFunction:  hex(theme.ColorAccent2),
		chroma.LiteralString: hex(theme.ColorOk),
		chroma.LiteralNumber: hex(theme.ColorOrange),
		chroma.Comment:       "italic " + hex(theme.ColorFgFaint),
		chroma.Operator:      hex(theme.ColorFgDim),
		chroma.Punctuation:   hex(theme.ColorFgDim),
	}))
}

func hex2(v uint32) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>4&0xf], digits[v&0xf]})
}

// editorTheme maps the Simmer palette onto the editor's chrome: line numbers,
// selection, the command line and the completion menu.
func editorTheme() goeditor.Theme {
	t := goeditor.DefaultTheme(true)
	fg := lipgloss.NewStyle().Foreground(theme.ColorFg)
	badge := func(bg color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Background(bg).Foreground(theme.ColorBg).Bold(true)
	}

	t.NormalModeStyle = badge(theme.ColorAccent)
	t.InsertModeStyle = badge(theme.ColorWarn)
	t.VisualModeStyle = badge(theme.ColorAccent2)
	t.CommandModeStyle = badge(theme.ColorOrange)
	t.SearchModeStyle = badge(theme.ColorInfo)

	t.StatusLineStyle = fg.Background(theme.ColorBg)
	t.CommandLineStyle = fg.Background(theme.ColorBg)
	t.MessageStyle = lipgloss.NewStyle().Foreground(theme.ColorOk)
	t.ErrorStyle = lipgloss.NewStyle().Foreground(theme.ColorErr).Bold(true)

	t.LineNumberStyle = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Width(4).Align(lipgloss.Right)
	t.CurrentLineNumberStyle = lipgloss.NewStyle().Foreground(theme.ColorAccent).Width(4).Align(lipgloss.Right).Bold(true)
	t.CurrentLineStyle = lipgloss.NewStyle()
	t.SelectionStyle = lipgloss.NewStyle().Background(theme.ColorTableSelBg)
	t.HighlightYankStyle = lipgloss.NewStyle().Background(theme.ColorAccent2).Foreground(theme.ColorBg)
	t.SearchHighlightStyle = lipgloss.NewStyle().Background(theme.ColorWarn).Foreground(theme.ColorBg)
	t.SearchInputPromptStyle = lipgloss.NewStyle().Foreground(theme.ColorWarn).Bold(true)
	t.SearchInputTextStyle = fg
	t.SearchInputCursorStyle = lipgloss.NewStyle().Foreground(theme.ColorWarn)
	t.PlaceholderStyle = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Italic(true)

	t.CompletionMenuItemStyle = fg.Background(theme.ColorTableHeader).Padding(0, 1)
	t.CompletionMenuSelectedItemStyle = lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorTableSelBg).Padding(0, 1).Bold(true)
	t.CompletionMenuBorderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColorBorderHi).Padding(0)
	t.CompletionMenuLabelStyle = lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	t.CompletionMenuTypeStyle = lipgloss.NewStyle().Foreground(theme.ColorFgDim)
	return t
}

// modeBadgeStyle colours the editor's mode in the viewer's status row. The
// editor's own status line is hidden; this badge replaces it.
func modeBadgeStyle(mode string) lipgloss.Style {
	bg := theme.ColorAccent // NORMAL
	switch mode {
	case "INSERT":
		bg = theme.ColorWarn
	case "VISUAL", "V-LINE":
		bg = theme.ColorAccent2
	case "COMMAND":
		bg = theme.ColorOrange
	case "SEARCH":
		bg = theme.ColorInfo
	}
	return lipgloss.NewStyle().Foreground(theme.ColorBg).Background(bg).Bold(true).Padding(0, 1)
}
