package dbviewer

import (
	"image/color"
	"os"
	"strings"

	"simmer/internal/theme"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// FileSavedMsg is returned after a save attempt completes.
type FileSavedMsg struct{ Err error }

type fileState int

const (
	fileStateNew   fileState = iota // file does not exist on disk yet
	fileStateClean                  // file exists and matches disk
	fileStateDirty                  // unsaved changes present
)

const defaultQueryFile = "untitled-1.sql"

// QueryPane renders the query head and the textarea editor in the right pane
// above the horizontal divider.
type QueryPane struct {
	activeTab int
	editor    textarea.Model
	rh        renderHelpers
	filePath  string
	state     fileState
}

func newQueryPane() QueryPane {
	ta := textarea.New()
	ta.Placeholder = "Write SQL here…"
	ta.ShowLineNumbers = true
	ta.Prompt = " "
	ta.EndOfBufferCharacter = 0
	ta.CharLimit = 0

	s := textarea.DefaultDarkStyles()
	s.Focused.Base = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Focused.Text = lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	s.Focused.LineNumber = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	s.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
	s.Focused.CursorLine = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Focused.Prompt = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	s.Blurred.Base = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)
	s.Blurred.LineNumber = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	s.Blurred.CursorLine = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.Prompt = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	ta.SetStyles(s)

	state := fileStateNew
	content := ""
	if data, err := os.ReadFile(defaultQueryFile); err == nil {
		content = string(data)
		state = fileStateClean
	}

	ta.SetValue(content)
	ta.Blur()

	return QueryPane{
		activeTab: 0,
		editor:    ta,
		rh:        newRenderHelpers(),
		filePath:  defaultQueryFile,
		state:     state,
	}
}

func (p QueryPane) FocusEditor() (QueryPane, tea.Cmd) {
	cmd := p.editor.Focus()
	return p, cmd
}

// Value returns the current editor content.
func (p QueryPane) Value() string { return p.editor.Value() }

// SetQuery replaces the editor content and moves the cursor to the end.
func (p QueryPane) SetQuery(sql string) QueryPane {
	p.editor.SetValue(sql)
	p.editor.MoveToEnd()
	p.state = fileStateDirty

	return p
}

func (p QueryPane) BlurEditor() QueryPane {
	p.editor.Blur()
	return p
}

// SaveCmd writes current editor content to disk. Called externally by the modal.
func (p QueryPane) SaveCmd() tea.Cmd {
	content := p.editor.Value()
	path := p.filePath

	return func() tea.Msg {
		err := os.WriteFile(path, []byte(content), 0o644)
		return FileSavedMsg{Err: err}
	}
}

func (p QueryPane) Update(msg tea.Msg) (QueryPane, tea.Cmd) {
	if sm, ok := msg.(FileSavedMsg); ok {
		if sm.Err == nil {
			p.state = fileStateClean
		}

		return p, nil
	}

	prev := p.editor.Value()
	var cmd tea.Cmd
	p.editor, cmd = p.editor.Update(msg)
	if p.editor.Value() != prev {
		p.state = fileStateDirty
	}

	return p, cmd
}

// Rows returns height strings each exactly width visual cells wide.
//
// Layout:
//
//	row 0        query head  "[q] Query  untitled-1.sql ● …  F5 run …"
//	row 1        ─── separator
//	rows 2..h-1  textarea
func (p QueryPane) Rows(width, height int, focused bool) []string {
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

func (p QueryPane) renderQueryHead(width int) string {
	rh := p.rh
	lbl := lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
	file := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	kS := lipgloss.NewStyle().Foreground(theme.ColorBorderHi).Background(theme.ColorBg).Bold(true)
	vS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)

	var dotColor color.Color
	switch p.state {
	case fileStateNew:
		dotColor = theme.ColorErr
	case fileStateClean:
		dotColor = theme.ColorOk
	case fileStateDirty:
		dotColor = theme.ColorWarn
	}

	dot := lipgloss.NewStyle().Foreground(dotColor).Background(theme.ColorBg).Render("●")

	left := lbl.PaddingLeft(1).Render("[q] Query") +
		rh.BlankN(2) +
		file.Render(p.filePath) +
		rh.BlankN(1) +
		dot

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
