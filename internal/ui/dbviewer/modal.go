package dbviewer

import (
	"context"
	"fmt"
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

// TablesLoadedMsg carries the result of an async table-list fetch.
type TablesLoadedMsg struct {
	Objects device.SQLiteObjects
	Err     error
}

// ColumnsLoadedMsg carries the result of an async column fetch for a table node.
type ColumnsLoadedMsg struct {
	node *explorerNode
	cols []device.ColumnInfo
	err  error
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
// returns a Cmd that asynchronously fetches the SQLite version and table list.
func (m *Modal) SetFile(dev device.Device, packageID, dbPath, dbName string) tea.Cmd {
	m.device = dev
	m.packageID = packageID
	m.dbPath = dbPath
	m.dbName = dbName
	m.sqliteVersion = ""

	if ep, ok := m.panes[paneSidebar].(explorerPane); ok {
		ep.inner.SetLoading()
		m.panes[paneSidebar] = ep
	}

	return tea.Batch(m.fetchSQLiteVersionCmd(), m.fetchTablesCmd())
}

func (m Modal) fetchTablesCmd() tea.Cmd {
	var (
		dev  = m.device
		pkg  = m.packageID
		path = m.dbPath
	)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		objs, err := device.ListSQLiteObjects(ctx, dev, pkg, path)
		return TablesLoadedMsg{Objects: objs, Err: err}
	}
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

func (m Modal) fetchColumnsCmd(node *explorerNode) tea.Cmd {
	var (
		dev   = m.device
		pkg   = m.packageID
		path  = m.dbPath
		table = node.label
	)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cols, err := device.QueryTableColumns(ctx, dev, pkg, path, table)
		return ColumnsLoadedMsg{node: node, cols: cols, err: err}
	}
}

// tryFetchColumns fires fetchColumnsCmd if the cursor is on an unloaded table node.
func (m Modal) tryFetchColumns() tea.Cmd {
	ep, ok := m.panes[paneSidebar].(explorerPane)
	if !ok {
		return nil
	}

	items := ep.inner.filteredVisibleItems()
	if ep.inner.cursor >= len(items) {
		return nil
	}

	node := items[ep.inner.cursor].node
	if node.kind != nodeKindTable {
		return nil
	}

	if node.loading || len(node.children) > 0 {
		return nil
	}

	node.loading = true

	return m.fetchColumnsCmd(node)
}

func buildColumnNodes(cols []device.ColumnInfo) []*explorerNode {
	nodes := make([]*explorerNode, len(cols))
	for i, c := range cols {
		var meta string
		if c.Type != "" {
			meta = strings.ToUpper(c.Type)
		}
		nodes[i] = &explorerNode{
			kind:  nodeKindCol,
			label: c.Name,
			meta:  meta,
			isPK:  c.IsPK,
			isFK:  c.IsFK,
		}
	}
	return nodes
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
		return fmt.Sprintf("SELECT * FROM %s;", ident(node.label)), true

	case nodeKindCol:
		if parentTable == "" {
			return "", false
		}

		return fmt.Sprintf("SELECT %s FROM %s;", ident(node.label), ident(parentTable)), true
	}

	return "", false
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

	if tm, ok := msg.(TablesLoadedMsg); ok {
		if tm.Err == nil {
			if ep, ok := m.panes[paneSidebar].(explorerPane); ok {
				ep.inner.SetTables(m.dbName, tm.Objects)
				m.panes[paneSidebar] = ep
			}
		}

		return m, nil
	}

	if cm, ok := msg.(ColumnsLoadedMsg); ok {
		cm.node.loading = false
		if cm.err == nil {
			cm.node.children = buildColumnNodes(cm.cols)
			cm.node.expanded = true
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

	case "enter":
		if m.focus == paneSidebar {
			if cmd := m.tryFetchColumns(); cmd != nil {
				return m, cmd
			}
		}
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)

		return m, cmd

	case "space":
		if m.focus == paneSidebar {
			if sql, ok := m.sidebarSQL(); ok {
				return m.injectQuery(sql)
			}
		}

		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)

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
