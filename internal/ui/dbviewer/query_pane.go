package dbviewer

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"simmer/internal/config"
	"simmer/internal/theme"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	goeditor "github.com/gusilva/goeditor"
	"github.com/gusilva/goeditor/core"
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

// QueryPane renders the query head and the SQL editor in the right pane
// above the horizontal divider.
type QueryPane struct {
	activeTab    int
	editor       goeditor.Model
	vim          bool // the editor is modal (normal/insert/…) rather than plain
	width        int  // pane size last applied to the editor
	height       int
	savePrompt   textinput.Model
	promptActive bool
	rh           renderHelpers
	filePath     string
	scriptDir    string // directory from config.ScriptPath; file is written to scriptDir/filePath
	state        fileState
	schema       sqlSchema // what completion offers
}

// queryHeadRows is the query head plus its separator, above the editor.
const queryHeadRows = 2

const (
	// completionDebounce is how long typing pauses before completion runs.
	completionDebounce = 120 * time.Millisecond
	// completionMenuItems is the most the menu lists at once; fewer when
	// the editor is too short to show that many with the menu's border.
	completionMenuItems = 8
)

func newQueryPane(cfg config.Config) QueryPane {
	// Simmer paints the terminal background itself, so the editor must not
	// ask the terminal: that would read stdin under Bubble Tea's feet.
	ed := goeditor.New(80, 10, goeditor.WithDarkBackground(true))
	ed.WithTheme(editorTheme())
	ed.SetLanguage(sqlLexer, sqlStyle)
	ed.HideStatusLine(true)
	ed.SetPlaceholder("Write SQL here…")
	ed.DisableVimMode(!cfg.VimMode)
	ed.WithCompletionAutoTrigger(true)
	ed.WithCompletionDebounce(completionDebounce)

	scriptDir := cfg.ScriptPath

	state := fileStateNew
	fullPath := filepath.Join(scriptDir, defaultQueryFile)
	if data, err := os.ReadFile(fullPath); err == nil {
		ed.SetContent(string(data))
		state = fileStateClean
	}
	ed.Blur()

	sp := textinput.New()
	sp.Placeholder = "path/to/query.sql"
	spStyles := textinput.DefaultDarkStyles()
	spStyles.Focused.Text = lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	spStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg)
	spStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	sp.SetStyles(spStyles)

	p := QueryPane{
		activeTab:  0,
		editor:     ed,
		vim:        cfg.VimMode,
		savePrompt: sp,
		rh:         newRenderHelpers(),
		filePath:   defaultQueryFile,
		scriptDir:  scriptDir,
		state:      state,
	}
	return p.withSize(80, 10+queryHeadRows)
}

func (p QueryPane) FocusEditor() (QueryPane, tea.Cmd) {
	p.editor.Focus()
	return p, nil
}

// Value returns the current editor content.
func (p QueryPane) Value() string { return p.editor.GetCurrentContent() }

// StatementUnderCursor returns the Statement under the editor's cursor; see
// statementAt for how a cursor between Statements is resolved.
func (p QueryPane) StatementUnderCursor() string {
	pos := p.editor.GetCursorPosition()
	return statementAt(p.Value(), pos.Row, pos.Col)
}

// LoadScript loads file content into the editor and marks the file as clean.
// filePath is just the base filename; scriptDir is kept from the current config.
func (p QueryPane) LoadScript(content, fileName string) QueryPane {
	p.editor.SetContent(content)
	_ = p.editor.SetCursorPositionEnd()
	p.filePath = fileName
	p.state = fileStateClean
	return p
}

// SetQuery replaces the editor content and moves the cursor to the end.
func (p QueryPane) SetQuery(sql string) QueryPane {
	p.editor.SetContent(sql)
	_ = p.editor.SetCursorPositionEnd()
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

// withVimMode switches the editor between modal (vim) and plain editing.
func (p QueryPane) withVimMode(on bool) QueryPane {
	if p.vim != on {
		p.editor.DisableVimMode(!on)
		p.vim = on
	}
	return p
}

// withSchema sets what completion offers.
func (p QueryPane) withSchema(s sqlSchema) QueryPane {
	p.schema = s
	return p
}

// CompletionMenuVisible reports whether the completion menu is open, so the
// keys it uses (Esc among them) go to it.
func (p QueryPane) CompletionMenuVisible() bool { return p.editor.IsCompletionMenuVisible() }

// VimMode reports whether the editor is modal.
func (p QueryPane) VimMode() bool { return p.vim }

// withSize sizes the editor to the pane, minus the query head. It has to
// happen in Update, not while rendering: the editor scrolls to keep the
// cursor visible against the size it was last given.
func (p QueryPane) withSize(width, height int) QueryPane {
	if p.width == width && p.height == height {
		return p
	}
	p.width, p.height = width, height
	editorH := max(height-queryHeadRows, 1)
	p.editor.SetSize(width, editorH)
	// The menu (items plus a two-row border) must fit above or below the
	// cursor line within the editor; the command line takes a row in vim.
	p.editor.SetCompletionMenuMaxVisibleItems(min(max(editorH-4, 1), completionMenuItems))
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
	content := p.Value()
	path := filepath.Join(p.scriptDir, p.filePath)

	return func() tea.Msg {
		if dir := filepath.Dir(path); dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		err := os.WriteFile(path, []byte(content), 0o644)
		return FileSavedMsg{Err: err}
	}
}

// editorMessageTTL is how long a message or error stays on the editor's
// command line.
const editorMessageTTL = 3 * time.Second

func (p QueryPane) Update(msg tea.Msg) (QueryPane, tea.Cmd) {
	switch msg := msg.(type) {
	case FileSavedMsg:
		if msg.Err != nil {
			return p, p.editor.DispatchError(msg.Err, editorMessageTTL)
		}
		p.state = fileStateClean
		return p, p.editor.DispatchMessage("written "+p.filePath, editorMessageTTL)

	case goeditor.SaveMsg:
		// :w writes the Script; :w name.sql writes it under a new name in
		// the script directory, as the save-as prompt does.
		if msg.Path != nil {
			if name := strings.TrimSpace(*msg.Path); name != "" {
				p.filePath = filepath.Base(name)
			}
		}
		return p, p.SaveCmd()

	case goeditor.ErrorMsg:
		return p, p.editor.DispatchError(msg.Error, editorMessageTTL)

	case goeditor.CompletionRequestMsg:
		// Built from the whole buffer, not the request's few context lines:
		// an alias is often defined after the cursor ("SELECT u.| FROM users u").
		pos := p.editor.GetCursorPosition()
		manual := msg.Context.TriggerKind == core.CompletionTriggerManual
		var items []core.Completion
		// The editor auto-triggers on the key that enters insert mode too
		// ("A", "o"): only a character that landed before the cursor counts.
		if manual || strings.HasSuffix(msg.Context.TextBeforeCursor, msg.Context.TriggerCharacter) {
			items = sqlCompletions(p.Value(), pos.Row, pos.Col, p.schema, manual)
		}
		p.editor.SetCompletions(items, msg.Context)
		// Update re-arms the editor's signal listener, which carries the
		// response back; without it the menu would wait for the next key.
		var cmd tea.Cmd
		p.editor, cmd = p.editor.Update(msg)
		return p, cmd

	case tea.PasteMsg:
		return p.paste(msg.Content)
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

	prev := p.Value()
	var cmd tea.Cmd
	p.editor, cmd = p.editor.Update(msg)

	if p.Value() != prev {
		p.state = fileStateDirty
	}

	return p, cmd
}

// paste types text into the editor at the cursor. The editor reads only its
// own clipboard, so a terminal paste is fed through as keys: that inserts it
// verbatim (no auto-indent) as one undo step, whatever mode the editor is in.
func (p QueryPane) paste(text string) (QueryPane, tea.Cmd) {
	if p.promptActive {
		var cmd tea.Cmd
		p.savePrompt, cmd = p.savePrompt.Update(tea.PasteMsg{Content: text})
		return p, cmd
	}
	if text == "" {
		return p, nil
	}

	core := p.editor.GetEditor()
	wasInsert := core.IsInsertMode()
	if !wasInsert {
		core.SetInsertMode()
	}
	var cmds []tea.Cmd
	for _, r := range strings.ReplaceAll(text, "\r\n", "\n") {
		var k tea.KeyPressMsg
		switch r {
		case '\n', '\r':
			k = tea.KeyPressMsg{Code: tea.KeyEnter}
		case '\t':
			k = tea.KeyPressMsg{Code: tea.KeyTab}
		default:
			k = tea.KeyPressMsg{Code: r, Text: string(r)}
		}
		var cmd tea.Cmd
		p.editor, cmd = p.editor.Update(k)
		cmds = append(cmds, cmd)
	}
	if !wasInsert {
		core.SetNormalMode()
	}
	p.state = fileStateDirty
	return p, tea.Batch(cmds...)
}

// modeLabel names the editor's current vim mode, or "" in plain editing.
func (p QueryPane) modeLabel() string {
	if !p.vim {
		return ""
	}
	switch {
	case p.editor.IsInsertMode():
		return "INSERT"
	case p.editor.IsVisualLineMode():
		return "V-LINE"
	case p.editor.IsVisualMode():
		return "VISUAL"
	case p.editor.IsCommandMode():
		return "COMMAND"
	case p.editor.IsSearchMode():
		return "SEARCH"
	}
	return "NORMAL"
}

// Rows returns height strings each exactly width visual cells wide.
//
// Layout:
//
//	row 0        query head  "[q] Query  untitled-1.sql ●
//	row 1        ─── separator
//	rows 2..h-1  editor
func (p QueryPane) Rows(width, height int, focused bool) []string {
	rh := p.rh
	if focused {
		rh = rh.WithSepColor(theme.ColorBorderHi)
	}
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

	editorH := height - queryHeadRows
	// Rendering works on a copy, so sizing here does not stick; Update-time
	// withSize is what the editor scrolls against. This only covers a
	// render before the first size arrives.
	p = p.withSize(width, height)

	editorLines := strings.Split(p.editor.View(), "\n")
	for i := range editorH {
		if i < len(editorLines) {
			out[i+queryHeadRows] = fillTo(editorLines[i])
		} else {
			out[i+queryHeadRows] = rh.BlankN(width)
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

	queryLabel := lblS.PaddingLeft(1).Render("[q] Query")
	fixedW := lipgloss.Width(queryLabel) + 2 + 1 + 1 // BlankN(2) + dot + BlankN(1)
	fileNameBudget := max(width-fixedW, 5)

	displayName := truncateLabel(p.filePath, fileNameBudget)

	left := queryLabel +
		rh.BlankN(2) +
		fileS.Render(displayName) +
		rh.BlankN(1) +
		dot

	return rh.ExactWidth(left, width)
}
