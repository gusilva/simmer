package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)


// ShowDBViewerMsg opens the database viewer overlay.
type ShowDBViewerMsg struct{}

const (
	dbPaneSidebar = 0
	dbPaneQuery   = 1
	dbPaneResults = 2
	dbPaneCount   = 3
)

// DBViewerModal is the full-screen database viewer overlay.
type DBViewerModal struct {
	width  int
	height int
	focus  int
	styles dbViewerStyles
	panes  [dbPaneCount]dbPane
}

func NewDBViewerModal() DBViewerModal {
	return DBViewerModal{
		focus:  dbPaneSidebar,
		styles: newDBViewerStyles(),
		panes: [dbPaneCount]dbPane{
			dbPaneSidebar: newDBExplorer().asPane(),
			dbPaneQuery:   newDBQueryPane().asPane(),
			dbPaneResults: newDBResultsPane().asPane(),
		},
	}
}

func (m *DBViewerModal) SetSize(w, h int) {
	m.width = w
	m.height = h
	l := computeDBViewerLayout(w, h)
	rp := m.panes[dbPaneResults].(resultsPane)
	m.panes[dbPaneResults] = resultsPane{rp.inner.withHeight(l.ResultsH)}
}

// setFocus transitions focus: blurs the old pane, focuses the new one.
func (m DBViewerModal) setFocus(idx int) (DBViewerModal, tea.Cmd) {
	if m.focus == idx {
		return m, nil
	}
	m.panes[m.focus] = m.panes[m.focus].Blur()
	m.focus = idx
	var cmd tea.Cmd
	m.panes[idx], cmd = m.panes[idx].Focus()
	return m, cmd
}

func (m DBViewerModal) Update(msg tea.Msg) (DBViewerModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Forward non-key messages (cursor blink, etc.) to focused pane.
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd
	}

	switch k.String() {
	case "esc", "q":
		return m, func() tea.Msg { return CancelOverlayMsg{} }

	case "tab":
		return m.setFocus((m.focus + 1) % dbPaneCount)

	case "shift+tab":
		return m.setFocus((m.focus + dbPaneCount - 1) % dbPaneCount)

	case "ctrl+l":
		return m.setFocus(dbPaneQuery)

	case "ctrl+j":
		if m.focus == dbPaneQuery {
			return m.setFocus(dbPaneResults)
		}

	case "ctrl+k":
		if m.focus == dbPaneResults {
			return m.setFocus(dbPaneQuery)
		}

	case "ctrl+h":
		if m.focus != dbPaneSidebar {
			return m.setFocus(dbPaneSidebar)
		}
		// Already on sidebar — pass through so filter input handles it as backspace.
		var cmd tea.Cmd
		m.panes[dbPaneSidebar], cmd = m.panes[dbPaneSidebar].Update(msg)
		return m, cmd

	default:
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd
	}

	return m, nil
}

// ScrimView returns a full-terminal-size overlay that simulates opacity.
// True alpha is not possible in terminals; ░ shade characters bleed the
// background colour through the pattern, approximating a semi-transparent dim.
func (m DBViewerModal) ScrimView() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	scrimS := lipgloss.NewStyle().Foreground(ColorBgDim).Background(ColorBg)
	row := scrimS.Render(strings.Repeat("░", m.width))
	rows := make([]string, m.height)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}
