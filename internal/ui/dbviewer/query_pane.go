package dbviewer

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"simmer/internal/config"
	"simmer/internal/theme"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
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
	activeTab    int
	editor       textarea.Model
	savePrompt   textinput.Model
	promptActive bool
	rh           renderHelpers
	filePath     string
	scriptDir    string // directory from config.ScriptPath; file is written to scriptDir/filePath
	state        fileState
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

	cfg, _ := config.Load()
	scriptDir := cfg.ScriptPath

	state := fileStateNew
	content := ""
	fullPath := filepath.Join(scriptDir, defaultQueryFile)
	if data, err := os.ReadFile(fullPath); err == nil {
		content = string(data)
		state = fileStateClean
	}

	ta.SetValue(content)
	ta.Blur()

	sp := textinput.New()
	sp.Placeholder = "path/to/query.sql"
	spStyles := textinput.DefaultDarkStyles()
	spStyles.Focused.Text = lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	spStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg)
	spStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	sp.SetStyles(spStyles)

	return QueryPane{
		activeTab:  0,
		editor:     ta,
		savePrompt: sp,
		rh:         newRenderHelpers(),
		filePath:   defaultQueryFile,
		scriptDir:  scriptDir,
		state:      state,
	}
}

func (p QueryPane) FocusEditor() (QueryPane, tea.Cmd) {
	cmd := p.editor.Focus()
	return p, cmd
}

// Value returns the current editor content.
func (p QueryPane) Value() string { return p.editor.Value() }

// StatementUnderCursor returns the SQL statement that contains the current
// cursor line. A statement is delimited by ";" characters. If the cursor sits
// on a blank line between statements the next statement is returned.
func (p QueryPane) StatementUnderCursor() string {
	return statementAtLine(p.editor.Value(), p.editor.Line())
}

// LoadScript loads file content into the editor and marks the file as clean.
// filePath is just the base filename; scriptDir is kept from the current config.
func (p QueryPane) LoadScript(content, fileName string) QueryPane {
	p.editor.SetValue(content)
	p.editor.MoveToEnd()
	p.filePath = fileName
	p.state = fileStateClean
	return p
}

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

func (p QueryPane) withScriptDir(dir string) QueryPane {
	p.scriptDir = dir
	return p
}

// PromptActive reports whether the save-as filename prompt is visible.
func (p QueryPane) PromptActive() bool { return p.promptActive }

// TriggerSave opens the save-as prompt pre-filled with the current filename.
// The user edits the filename only; scriptDir from config is always used as the
// save directory.
func (p QueryPane) TriggerSave() (QueryPane, tea.Cmd) {
	p.savePrompt.SetValue(p.filePath)
	p.savePrompt.CursorEnd()
	cmd := p.savePrompt.Focus()
	p.promptActive = true
	return p, cmd
}

// confirmSave stores the filename the user typed and writes the file to scriptDir.
func (p QueryPane) confirmSave() (QueryPane, tea.Cmd) {
	name := strings.TrimSpace(p.savePrompt.Value())
	if name == "" {
		return p, nil
	}
	// Keep scriptDir from config; only the filename is user-editable here.
	p.filePath = filepath.Base(name)
	p.promptActive = false
	p.savePrompt.Blur()
	return p, p.SaveCmd()
}

// SaveCmd writes current editor content to scriptDir/filePath.
func (p QueryPane) SaveCmd() tea.Cmd {
	content := p.editor.Value()
	path := filepath.Join(p.scriptDir, p.filePath)

	return func() tea.Msg {
		if dir := filepath.Dir(path); dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
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

	if p.promptActive {
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "enter":
				return p.confirmSave()
			case "esc":
				p.promptActive = false
				p.savePrompt.Blur()
				return p, nil
			}
		}
		var cmd tea.Cmd
		p.savePrompt, cmd = p.savePrompt.Update(msg)
		return p, cmd
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
	kS := lipgloss.NewStyle().Foreground(theme.ColorBorderHi).Background(theme.ColorBg).Bold(true)
	vS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)

	if p.promptActive {
		accentS := lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
		faintS := lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)

		hints := kS.Render("Enter") + vS.Render(" save") + rh.BlankN(2) + kS.Render("Esc") + vS.Render(" cancel")
		hintsW := lipgloss.Width(hints)

		// Minimum input width; hints are dropped if the row is too narrow.
		const minInput = 12
		const gap = 2 // spaces between input and hints

		prefix := accentS.PaddingLeft(1).Render("Save as")
		prefixW := lipgloss.Width(prefix)

		dir := p.scriptDir
		if dir == "" {
			dir = "./"
		}

		// Budget available for the dir annotation + input area.
		// layout: prefix + " (" + dir + "):  " + [input] + gap + hints
		// We need at least minInput + gap + hintsW of space after the prefix.
		remaining := width - prefixW
		showHints := remaining >= minInput+gap+hintsW
		if !showHints {
			hintsW = 0
			hints = ""
		}
		// Space consumed by the dir annotation: " (" + dir + "):  " = len(dir)+6
		// Reserve minInput for the textinput itself.
		dirBudget := remaining - minInput - 2 // 2 = gap between input and hints (or edge)
		if showHints {
			dirBudget -= hintsW + gap
		}
		dirAnnotation := " (" + dir + "):  "
		if dirBudget < 6 {
			// No room even for a short dir; omit it entirely.
			dirAnnotation = ":  "
		} else if len([]rune(dirAnnotation)) > dirBudget {
			// Truncate the dir, keep the surrounding punctuation.
			maxDir := dirBudget - 6 // " (" + "…" + "):  "
			if maxDir > 0 {
				runes := []rune(dir)
				if len(runes) > maxDir {
					dir = "…" + string(runes[len(runes)-maxDir:])
				}
			}
			dirAnnotation = " (" + dir + "):  "
		}

		lbl := prefix + faintS.Render(dirAnnotation)
		lblW := lipgloss.Width(lbl)

		inputW := width - lblW - gap
		if showHints {
			inputW -= hintsW + gap
		}
		if inputW < minInput {
			inputW = minInput
		}
		p.savePrompt.SetWidth(inputW)

		line := lbl + p.savePrompt.View()
		if showHints {
			line += rh.BlankN(gap) + hints
		}
		return rh.ExactWidth(line, width)
	}

	lblS := lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
	fileS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)

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

	hints := []string{
		kS.Render("F5") + vS.Render(" run stmt"),
		kS.Render("^Enter") + vS.Render(" run all"),
		kS.Render("⌘S") + vS.Render(" save"),
		kS.Render("⇥") + vS.PaddingRight(1).Render(" complete"),
	}
	right := strings.Join(hints, rh.BlankN(2))

	// Fixed prefix: " [q] Query  " + dot + " "
	queryLabel := lblS.PaddingLeft(1).Render("[q] Query")
	fixedW := lipgloss.Width(queryLabel) + 2 + 1 + 1 // BlankN(2) + dot + BlankN(1)
	rightW := lipgloss.Width(right)
	fileNameBudget := max(width-fixedW-rightW-1, 5) // 1 = minimum gap

	displayName := truncateLabel(p.filePath, fileNameBudget)

	left := queryLabel +
		rh.BlankN(2) +
		fileS.Render(displayName) +
		rh.BlankN(1) +
		dot

	gap := max(width-lipgloss.Width(left)-rightW, 1)
	return rh.ExactWidth(left+rh.BlankN(gap)+right, width)
}

// statementAtLine extracts the SQL statement that contains cursorLine (0-indexed)
// from text. Statements are delimited by ";". The search scans backward for the
// previous ";" then forward for the next ";" to find the boundaries.
func statementAtLine(text string, cursorLine int) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return strings.TrimSpace(text)
	}
	if cursorLine < 0 {
		cursorLine = 0
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
	}

	// Scan backward from the line before cursor to find where the current
	// statement starts (the line after the previous ";").
	start := 0
	for i := cursorLine - 1; i >= 0; i-- {
		if strings.Contains(lines[i], ";") {
			start = i + 1
			break
		}
	}

	// Scan forward from cursor to find where the current statement ends (the
	// line that contains the next ";").
	end := len(lines) - 1
	for i := cursorLine; i < len(lines); i++ {
		if strings.Contains(lines[i], ";") {
			end = i
			break
		}
	}

	return strings.TrimSpace(strings.Join(lines[start:end+1], "\n"))
}
