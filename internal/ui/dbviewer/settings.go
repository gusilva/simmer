package dbviewer

import (
	"strings"

	"simmer/internal/config"
	"simmer/internal/theme"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type settingsFocus int

const (
	settingsFocusScript settingsFocus = iota
	settingsFocusQuery
	settingsFocusCancel
	settingsFocusSave
	settingsFocusCount = 4
)

// Settings is the in-overlay settings form shown when the user presses Ctrl+S.
type Settings struct {
	dbName       string // database this settings form is editing
	scriptPath   textinput.Model
	tablesQuery  textarea.Model
	focus        settingsFocus
	rh           renderHelpers
	picker       filepicker.Model
	pickerActive bool
}

func newSettings(cfg config.Config, dbName string) Settings {
	dbcfg := cfg.ForDB(dbName)

	si := textinput.New()
	si.Placeholder = "./"
	si.SetValue(dbcfg.ScriptPath)
	si.SetWidth(60)
	_ = si.Focus()

	tq := textarea.New()
	tq.Placeholder = "SELECT type, name FROM sqlite_master…"
	tq.SetValue(dbcfg.TablesQuery)
	tq.SetWidth(60)
	tq.SetHeight(3)
	tq.ShowLineNumbers = false
	tq.Blur()

	s := textarea.DefaultDarkStyles()
	s.Focused.Base = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Focused.Text = lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	s.Focused.CursorLine = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Focused.Prompt = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Focused.EndOfBuffer = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.Base = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.Text = lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)
	s.Blurred.CursorLine = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.Prompt = lipgloss.NewStyle().Background(theme.ColorBg)
	s.Blurred.EndOfBuffer = lipgloss.NewStyle().Background(theme.ColorBg)
	tq.SetStyles(s)

	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	startDir := dbcfg.ScriptPath
	if startDir == "" {
		startDir = "."
	}
	fp.CurrentDirectory = startDir

	fpStyles := filepicker.DefaultStyles()
	fpStyles.Cursor = lipgloss.NewStyle().Foreground(theme.ColorAccent)
	fpStyles.Directory = lipgloss.NewStyle().Foreground(theme.ColorAccent2)
	fpStyles.DisabledFile = lipgloss.NewStyle().Foreground(theme.ColorFgFaint)
	fpStyles.Selected = lipgloss.NewStyle().Foreground(theme.ColorAccent).Bold(true)
	fpStyles.EmptyDirectory = lipgloss.NewStyle().Foreground(theme.ColorFgFaint).PaddingLeft(2).SetString("No directories found.")
	fp.Styles = fpStyles

	return Settings{
		dbName:      dbName,
		scriptPath:  si,
		tablesQuery: tq,
		focus:       settingsFocusScript,
		rh:          newRenderHelpers(),
		picker:      fp,
	}
}

func (s Settings) currentDBConfig() config.DBConfig {
	return config.DBConfig{
		ScriptPath:  s.scriptPath.Value(),
		TablesQuery: s.tablesQuery.Value(),
	}
}

func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	// Picker mode: route all input there except Space (select) and Esc (cancel).
	if s.pickerActive {
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "esc":
				s.pickerActive = false
				return s, nil
			case "space", "enter":
				// Select the directory the picker is currently browsing.
				dir := s.picker.CurrentDirectory
				s.scriptPath.SetValue(dir)
				s.pickerActive = false
				return s, nil
			}
		}
		var cmd tea.Cmd
		s.picker, cmd = s.picker.Update(msg)
		return s, cmd
	}

	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "tab":
			return s.cycleForward()
		case "shift+tab":
			return s.cycleBackward()
		case "esc":
			return s, func() tea.Msg { return SettingsCancelMsg{} }
		case "super+s":
			return s, saveDBConfigCmd(s.dbName, s.currentDBConfig())
		case "enter":
			if s.focus == settingsFocusSave {
				return s, saveDBConfigCmd(s.dbName, s.currentDBConfig())
			}
			if s.focus == settingsFocusCancel {
				return s, func() tea.Msg { return SettingsCancelMsg{} }
			}
			if s.focus == settingsFocusScript {
				// Open filepicker starting from currently typed path.
				dir := strings.TrimSpace(s.scriptPath.Value())
				if dir == "" {
					dir = "."
				}
				s.picker.CurrentDirectory = dir
				s.pickerActive = true
				return s, s.picker.Init()
			}
		}
	}

	var cmd tea.Cmd
	switch s.focus {
	case settingsFocusScript:
		s.scriptPath, cmd = s.scriptPath.Update(msg)
	case settingsFocusQuery:
		s.tablesQuery, cmd = s.tablesQuery.Update(msg)
	}
	return s, cmd
}

func (s Settings) cycleForward() (Settings, tea.Cmd) {
	return s.applyFocus(settingsFocus((int(s.focus) + 1) % settingsFocusCount))
}

func (s Settings) cycleBackward() (Settings, tea.Cmd) {
	return s.applyFocus(settingsFocus((int(s.focus) + settingsFocusCount - 1) % settingsFocusCount))
}

func (s Settings) applyFocus(f settingsFocus) (Settings, tea.Cmd) {
	s.focus = f
	s.scriptPath.Blur()
	s.tablesQuery.Blur()
	switch f {
	case settingsFocusScript:
		cmd := s.scriptPath.Focus()
		return s, cmd
	case settingsFocusQuery:
		cmd := s.tablesQuery.Focus()
		return s, cmd
	}
	return s, nil
}

// Rows renders the settings form into height strings each width cells wide.
func (s *Settings) Rows(width, height int) []string {
	rh := s.rh
	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}
	if height < 8 || width < 24 {
		return out
	}

	if s.pickerActive {
		return s.pickerRows(width, height)
	}

	const leftPad = 3
	fieldW := width - leftPad*2
	contentW := max(fieldW-2, 10) // inside border chars

	s.scriptPath.SetWidth(contentW)
	s.tablesQuery.SetWidth(contentW)

	fgS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	accentS := lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
	faintS := lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	dimS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)
	btnNS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBorder).Padding(0, 1)
	btnAS := lipgloss.NewStyle().Foreground(theme.ColorBg).Background(theme.ColorAccent).Bold(true).Padding(0, 1)

	fillTo := func(line string) string { return rh.FillTo(line, width) }
	pad := func(line string) string { return fillTo(rh.BlankN(leftPad) + line) }

	borderFor := func(focused bool) lipgloss.Style {
		c := theme.ColorBorder
		if focused {
			c = theme.ColorAccent
		}
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c).
			Width(contentW)
	}

	row := 0
	emit := func(line string) {
		if row < height {
			out[row] = line
			row++
		}
	}

	emit(fillTo(rh.BlankN(leftPad) + accentS.Render("Settings")))
	emit(rh.Sep(width))
	emit(rh.BlankN(width))

	// Script Path
	{
		lbl := fgS.Render("Script Path")
		hint := ""
		if s.focus == settingsFocusScript {
			lbl = accentS.Render("Script Path")
			hint = faintS.Render("  Enter to browse")
		}
		emit(pad(lbl + hint))
		box := borderFor(s.focus == settingsFocusScript).Render(s.scriptPath.View())
		for _, l := range strings.Split(box, "\n") {
			emit(pad(l))
		}
	}

	emit(rh.BlankN(width))

	// Tables List Query
	{
		lbl := fgS.Render("Tables List Query")
		if s.focus == settingsFocusQuery {
			lbl = accentS.Render("Tables List Query")
		}
		emit(pad(lbl + faintS.Render("  SQL - ensure query returns a 'name' and 'custom_table_name' columns")))
		box := borderFor(s.focus == settingsFocusQuery).Render(s.tablesQuery.View())
		for _, l := range strings.Split(box, "\n") {
			emit(pad(l))
		}
	}

	// Push buttons to bottom, leaving one blank row above them.
	btnRow := height - 2
	if row < btnRow {
		row = btnRow
	}

	// hint line
	out[height-1] = fillTo(rh.BlankN(leftPad) +
		dimS.Render("Tab") + faintS.Render(" next  ") +
		dimS.Render("⌘S") + faintS.Render(" save  ") +
		dimS.Render("Esc") + faintS.Render(" cancel"))

	cancelS := btnNS
	saveS := btnNS
	if s.focus == settingsFocusCancel {
		cancelS = btnAS
	}
	if s.focus == settingsFocusSave {
		saveS = btnAS
	}
	buttons := cancelS.Render("Cancel") + rh.BlankN(2) + saveS.Render("Save")
	gap := max(width-lipgloss.Width(buttons)-leftPad, 1)
	out[btnRow] = fillTo(rh.BlankN(gap) + buttons)

	_ = dimS

	return out
}

// pickerRows renders the directory picker overlay in place of the settings form.
func (s *Settings) pickerRows(width, height int) []string {
	rh := s.rh
	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}

	accentS := lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
	faintS := lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	dimS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)
	dirS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	fillTo := func(line string) string { return rh.FillTo(line, width) }
	const leftPad = 2

	// Fixed header rows.
	const headerRows = 3 // title, sep, current-dir
	// Fixed footer rows.
	const footerRows = 2 // sep, hints

	// How many rows the picker occupies.
	pickerH := height - headerRows - footerRows
	if pickerH < 1 {
		pickerH = 1
	}

	// Header.
	out[0] = fillTo(rh.BlankN(leftPad) + accentS.Render("Pick Script Folder"))
	out[1] = rh.Sep(width)
	out[2] = fillTo(rh.BlankN(leftPad) + faintS.Render("in ") + dirS.Render(s.picker.CurrentDirectory))

	// Picker body — split view into individual lines.
	pickerLines := strings.Split(s.picker.View(), "\n")
	for i := range pickerH {
		outRow := headerRows + i
		if outRow >= height-footerRows {
			break
		}
		if i < len(pickerLines) {
			out[outRow] = fillTo(pickerLines[i])
		}
	}

	// Footer.
	out[height-2] = rh.Sep(width)
	out[height-1] = fillTo(rh.BlankN(leftPad) +
		dimS.Render("j/k") + faintS.Render(" navigate  ") +
		dimS.Render("l/enter") + faintS.Render(" open  ") +
		dimS.Render("space") + faintS.Render(" select  ") +
		dimS.Render("esc") + faintS.Render(" cancel"))

	return out
}
