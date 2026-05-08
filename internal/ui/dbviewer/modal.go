package dbviewer

import (
	"context"
	"strings"
	"time"

	"simmer/internal/device"
	"simmer/internal/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ShowMsg opens the database viewer overlay.
type ShowMsg struct{}

// SQLiteVersionMsg carries the result of an async SQLite version lookup.
type SQLiteVersionMsg struct {
	Version string
	Err     error
}

const (
	paneSidebar = 0
	paneQuery   = 1
	paneResults = 2
	paneCount   = 3
)

// Modal is the full-screen database viewer overlay.
type Modal struct {
	width   int
	height  int
	focus   int
	styles  viewerStyles
	panes   [paneCount]pane
	onClose func() tea.Msg

	device        device.Device
	packageID     string
	dbPath        string
	dbName        string
	sqliteVersion string
}

// New creates a Modal. onClose is called when the user dismisses the overlay;
// it should return the appropriate close message for the parent application.
func New(onClose func() tea.Msg) Modal {
	return Modal{
		focus:  paneSidebar,
		styles: newViewerStyles(),
		panes: [paneCount]pane{
			paneSidebar: newExplorer().asPane(),
			paneQuery:   newQueryPane().asPane(),
			paneResults: newResultsPane().asPane(),
		},
		onClose: onClose,
	}
}

// SetFile attaches a database file and its owning device to the viewer and
// returns a Cmd that asynchronously fetches the SQLite version.
func (m *Modal) SetFile(dev device.Device, packageID, dbPath, dbName string) tea.Cmd {
	m.device = dev
	m.packageID = packageID
	m.dbPath = dbPath
	m.dbName = dbName
	m.sqliteVersion = ""

	return m.fetchSQLiteVersionCmd()
}

func (m Modal) fetchSQLiteVersionCmd() tea.Cmd {
	var (
		dev = m.device
		pkg = m.packageID
	)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		ver, err := device.SQLiteVersion(ctx, dev, pkg)

		return SQLiteVersionMsg{Version: ver, Err: err}
	}
}

func (m *Modal) SetSize(w, h int) {
	m.width = w
	m.height = h
	l := computeLayout(w, h)
	rp := m.panes[paneResults].(resultsPaneAdapter)
	m.panes[paneResults] = resultsPaneAdapter{rp.inner.withHeight(l.ResultsH)}
}

func (m Modal) setFocus(idx int) (Modal, tea.Cmd) {
	if m.focus == idx {
		return m, nil
	}
	m.panes[m.focus] = m.panes[m.focus].Blur()
	m.focus = idx
	var cmd tea.Cmd
	m.panes[idx], cmd = m.panes[idx].Focus()
	return m, cmd
}

func (m Modal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	if vm, ok := msg.(SQLiteVersionMsg); ok {
		if vm.Err == nil {
			m.sqliteVersion = vm.Version
		}

		return m, nil
	}

	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd
	}

	switch k.String() {
	case "esc", "q":
		return m, m.onClose

	case "tab":
		return m.setFocus((m.focus + 1) % paneCount)

	case "shift+tab":
		return m.setFocus((m.focus + paneCount - 1) % paneCount)

	case "ctrl+l":
		return m.setFocus(paneQuery)

	case "ctrl+j":
		if m.focus == paneQuery {
			return m.setFocus(paneResults)
		}

	case "ctrl+k":
		if m.focus == paneResults {
			return m.setFocus(paneQuery)
		}

	case "ctrl+h":
		if m.focus != paneSidebar {
			return m.setFocus(paneSidebar)
		}
		var cmd tea.Cmd
		m.panes[paneSidebar], cmd = m.panes[paneSidebar].Update(msg)
		return m, cmd

	default:
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd
	}

	return m, nil
}

// ScrimView returns a full-terminal-size overlay that simulates opacity.
func (m Modal) ScrimView() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	scrimS := lipgloss.NewStyle().Foreground(theme.ColorBgDim).Background(theme.ColorBg)
	row := scrimS.Render(strings.Repeat("░", m.width))
	rows := make([]string, m.height)
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, "\n")
}
