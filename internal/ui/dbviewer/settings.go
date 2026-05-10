package dbviewer

import (
	"strings"

	"simmer/internal/config"
	"simmer/internal/theme"

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
	scriptPath  textinput.Model
	tablesQuery textarea.Model
	focus       settingsFocus
	rh          renderHelpers
}

func newSettings(cfg config.Config) Settings {
	si := textinput.New()
	si.Placeholder = "./"
	si.SetValue(cfg.ScriptPath)
	si.SetWidth(60)
	_ = si.Focus()

	tq := textarea.New()
	tq.Placeholder = "SELECT type, name FROM sqlite_master…"
	tq.SetValue(cfg.TablesQuery)
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

	return Settings{
		scriptPath:  si,
		tablesQuery: tq,
		focus:       settingsFocusScript,
		rh:          newRenderHelpers(),
	}
}

func (s Settings) currentConfig() config.Config {
	return config.Config{
		ScriptPath:  s.scriptPath.Value(),
		TablesQuery: s.tablesQuery.Value(),
	}
}

func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "tab":
			return s.cycleForward()
		case "shift+tab":
			return s.cycleBackward()
		case "esc":
			return s, func() tea.Msg { return SettingsCancelMsg{} }
		case "super+s":
			return s, saveConfigCmd(s.currentConfig())
		case "enter":
			if s.focus == settingsFocusSave {
				return s, saveConfigCmd(s.currentConfig())
			}
			if s.focus == settingsFocusCancel {
				return s, func() tea.Msg { return SettingsCancelMsg{} }
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
		if s.focus == settingsFocusScript {
			lbl = accentS.Render("Script Path")
		}
		emit(pad(lbl))
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
		emit(pad(lbl + faintS.Render("  SQL")))
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
