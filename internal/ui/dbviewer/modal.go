package dbviewer

import (
	"fmt"
	"strings"
	"time"

	"simmer/internal/config"
	"simmer/internal/device"
	"simmer/internal/logging"
	"simmer/internal/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	paneSidebar = 0
	paneQuery   = 1
	paneResults = 2
	paneCount   = 3
)

// queryStats holds the outcome metrics of the last executed query.
type queryStats struct {
	elapsed  time.Duration
	rowCount int
	colCount int
	hasData  bool
}

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
	tablesQuery   string // from config; used by fetchTablesCmd
	scriptDir     string // from config; used by fetchScriptsCmd
	logger        *logging.Logger

	lastStats      queryStats
	queryStartedAt time.Time

	statusMsg   string // transient message shown in the status bar
	statusIsErr bool   // true → render statusMsg in error color

	settingsOpen bool
	settings     Settings
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
// returns a Cmd that asynchronously fetches the SQLite version and table list.
func (m *Modal) SetFile(dev device.Device, packageID, dbPath, dbName string, logger *logging.Logger) tea.Cmd {
	m.logger = logger
	m.device = dev
	m.packageID = packageID
	m.dbPath = dbPath
	m.dbName = dbName
	m.sqliteVersion = ""

	cfg, _ := config.Load()
	dbcfg := cfg.ForDB(dbName)
	m.tablesQuery = dbcfg.EffectiveTablesQuery()
	m.scriptDir = dbcfg.ScriptPath

	if qp, ok := m.panes[paneQuery].(queryPaneAdapter); ok {
		m.panes[paneQuery] = queryPaneAdapter{qp.inner.withScriptDir(dbcfg.ScriptPath)}
	}

	if ep, ok := m.panes[paneSidebar].(explorerPane); ok {
		ep.inner.SetLoading()
		m.panes[paneSidebar] = ep
	}

	return tea.Batch(m.fetchSQLiteVersionCmd(), m.fetchTablesCmd(), m.fetchScriptsCmd())
}

func (m *Modal) SetSize(w, h int) {
	m.width = w
	m.height = h
	l := computeLayout(w, h)
	rp := m.panes[paneResults].(resultsPaneAdapter)
	m.panes[paneResults] = resultsPaneAdapter{rp.inner.withHeight(l.ResultsH)}
	if m.settingsOpen && m.settings.pickerActive {
		m.settings.picker.SetHeight(m.pickerBodyH())
	}
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

// setQueryText sets the query editor content without changing focus.
func (m Modal) setQueryText(sql string) Modal {
	qp, ok := m.panes[paneQuery].(queryPaneAdapter)
	if !ok {
		return m
	}
	qp.inner = qp.inner.SetQuery(sql)
	m.panes[paneQuery] = qp
	return m
}

// injectQuery sets the query editor content and moves focus to the query pane.
func (m Modal) injectQuery(sql string) (Modal, tea.Cmd) {
	qp, ok := m.panes[paneQuery].(queryPaneAdapter)
	if !ok {
		return m, nil
	}
	qp.inner = qp.inner.SetQuery(sql)
	m.panes[paneQuery] = qp
	return m.setFocus(paneQuery)
}

// sidebarSQL returns the SQL string to inject for the currently selected
// sidebar node, and true when the node is a table or column.
func (m Modal) sidebarSQL() (string, bool) {
	ep, ok := m.panes[paneSidebar].(explorerPane)
	if !ok {
		return "", false
	}

	node, parentTable := ep.inner.selectedNodeInfo()
	if node == nil {
		return "", false
	}

	ident := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	switch node.kind {
	case nodeKindTable:
		return fmt.Sprintf("SELECT * FROM %s;", ident(node.realNameOrLabel())), true
	case nodeKindCol:
		if parentTable == "" {
			return "", false
		}
		return fmt.Sprintf("SELECT %s FROM %s;", ident(node.label), ident(parentTable)), true
	}

	return "", false
}

// sidebarScript returns the full path and filename when the cursor is on a script node.
func (m Modal) sidebarScript() (path, name string, ok bool) {
	ep, epOK := m.panes[paneSidebar].(explorerPane)
	if !epOK {
		return "", "", false
	}
	node, _ := ep.inner.selectedNodeInfo()
	if node == nil || node.kind != nodeKindScript {
		return "", "", false
	}
	return node.realName, node.label, true
}

// openSettings loads the persisted config and opens the settings form for the current database.
func (m Modal) openSettings() Modal {
	cfg, _ := config.Load()
	m.settings = newSettings(cfg, m.dbName)
	m.settingsOpen = true
	m.settings.picker.SetHeight(m.pickerBodyH())
	return m
}

// pickerBodyH returns the number of rows available for the filepicker list.
func (m Modal) pickerBodyH() int {
	l := computeLayout(m.width, m.height)
	const headerRows = 3 // title + sep + current-dir
	const footerRows = 2 // sep + hints
	h := l.BodyH - headerRows - footerRows
	if h < 1 {
		h = 1
	}
	return h
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
