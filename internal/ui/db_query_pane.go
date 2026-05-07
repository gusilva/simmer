package ui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DBQueryPane renders the query head and the textarea editor in the right pane
// above the horizontal divider.
type DBQueryPane struct {
	activeTab int
	editor    textarea.Model
	rh        dbRenderHelpers
}

func newDBQueryPane() DBQueryPane {
	ta := textarea.New()
	ta.Placeholder = "Write SQL here…"
	ta.ShowLineNumbers = true
	ta.Prompt = " "
	ta.EndOfBufferCharacter = 0
	ta.CharLimit = 0

	s := textarea.DefaultDarkStyles()
	s.Focused.Base = lipgloss.NewStyle().Background(ColorBg)
	s.Focused.Text = lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	s.Focused.LineNumber = lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	s.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	s.Focused.CursorLine = lipgloss.NewStyle().Background(ColorBg)
	s.Focused.Prompt = lipgloss.NewStyle().Background(ColorBg)
	s.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	s.Blurred.Base = lipgloss.NewStyle().Background(ColorBg)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	s.Blurred.LineNumber = lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	s.Blurred.CursorLine = lipgloss.NewStyle().Background(ColorBg)
	s.Blurred.Prompt = lipgloss.NewStyle().Background(ColorBg)
	s.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	ta.SetStyles(s)

	ta.SetValue(strings.Join([]string{
		"-- Recently booted devices, their runtime & installed app counts",
		"SELECT  d.udid, d.name, d.os, d.state, r.version AS runtime,",
		"        count(a.id) AS apps, d.cpu_pct, d.booted_at",
		"FROM    devices d",
		"LEFT JOIN device_runtimes r ON r.runtime_id = d.runtime_id",
		"LEFT JOIN apps            a ON a.device_udid = d.udid",
		"WHERE   d.state = 'Booted' AND d.booted_at >= datetime('now', '-7 days')",
		"GROUP BY d.udid ORDER BY d.booted_at DESC LIMIT 250;",
	}, "\n"))

	// start blurred; Focus() is called when the user presses Ctrl+L
	ta.Blur()

	return DBQueryPane{activeTab: 0, editor: ta, rh: newDBRenderHelpers()}
}

// FocusEditor focuses the textarea and returns the blink cmd.
func (p DBQueryPane) FocusEditor() (DBQueryPane, tea.Cmd) {
	cmd := p.editor.Focus()
	return p, cmd
}

// BlurEditor blurs the textarea.
func (p DBQueryPane) BlurEditor() DBQueryPane {
	p.editor.Blur()
	return p
}

// Update forwards messages to the textarea (cursor blink, keystrokes, etc.).
func (p DBQueryPane) Update(msg tea.Msg) (DBQueryPane, tea.Cmd) {
	var cmd tea.Cmd
	p.editor, cmd = p.editor.Update(msg)
	return p, cmd
}

// Rows returns height strings each exactly width visual cells wide.
//
// Layout:
//
//	row 0        query head  "[q] Query  untitled-1.sql ● …  F5 run …"
//	row 1        ─── separator
//	rows 2..h-1  textarea
func (p DBQueryPane) Rows(width, height int, focused bool) []string {
	rh := p.rh
	fillTo := func(s string) string { return rh.FillTo(s, width) }

	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}
	if height == 0 || width == 0 {
		return out
	}

	out[0] = fillTo(p.renderQueryHead(width))
	if height == 1 {
		return out
	}

	out[1] = rh.Sep(width)
	if height <= 2 {
		return out
	}

	// resize on the local copy so View() produces the right dimensions
	editorH := height - 2
	p.editor.SetWidth(width)
	p.editor.SetHeight(editorH)

	editorLines := strings.Split(p.editor.View(), "\n")
	for i := range editorH {
		if i < len(editorLines) {
			out[i+2] = fillTo(editorLines[i])
		} else {
			out[i+2] = rh.BlankN(width)
		}
	}
	return out
}

// ── Query head ────────────────────────────────────────────────────────────────

func (p DBQueryPane) renderQueryHead(width int) string {
	rh    := p.rh
	lbl   := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	file  := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	dot   := lipgloss.NewStyle().Foreground(ColorWarn).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	kS    := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	vS    := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	left := lbl.PaddingLeft(1).Render("[q] Query") +
		rh.BlankN(2) +
		file.Render("untitled-1.sql") +
		rh.BlankN(1) +
		dot.Render("●") +
		faint.Render(" — 8 lines")

	hints := []string{
		kS.Render("F5") + vS.Render(" run"),
		kS.Render("^Enter") + vS.Render(" run line"),
		kS.Render("^S") + vS.Render(" save"),
		kS.Render("⇥") + vS.PaddingRight(1).Render(" complete"),
	}
	right := strings.Join(hints, rh.BlankN(2))

	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + rh.BlankN(gap) + right
}
