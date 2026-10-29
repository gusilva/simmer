package dbviewer

import (
	"context"
	"os"
	"strings"

	"simmer/internal/device"
	"simmer/internal/session"

	tea "charm.land/bubbletea/v2"
)

// target identifies the database this modal is attached to, for the Session
// calls below.
func (m Modal) target() session.DBTarget {
	return session.DBTarget{
		Device:    m.device,
		PackageID: m.packageID,
		Path:      m.dbPath,
		Name:      m.dbName,
	}
}

func (m Modal) fetchTablesCmd() tea.Cmd {
	sess, target := m.sess, m.target()
	return func() tea.Msg {
		objs, err := sess.SchemaObjects(context.Background(), target)
		return TablesLoadedMsg{Objects: objs, Err: err}
	}
}

func (m Modal) fetchSQLiteVersionCmd() tea.Cmd {
	sess, target := m.sess, m.target()
	return func() tea.Msg {
		ver, err := sess.SQLiteVersion(context.Background(), target)
		return SQLiteVersionMsg{Version: ver, Err: err}
	}
}

func (m Modal) fetchColumnsCmd(node *explorerNode) tea.Cmd {
	sess, target := m.sess, m.target()
	table := node.realNameOrLabel()
	return func() tea.Msg {
		cols, err := sess.TableColumns(context.Background(), target, table)
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

// executeQueryCmd runs the SQL statement under the cursor.
func (m Modal) executeQueryCmd() tea.Cmd {
	qp, ok := m.panes[paneQuery].(queryPaneAdapter)
	if !ok {
		return nil
	}
	return m.runQuery(qp.inner.StatementUnderCursor())
}

// executeAllCmd runs the entire editor content as one query.
func (m Modal) executeAllCmd() tea.Cmd {
	qp, ok := m.panes[paneQuery].(queryPaneAdapter)
	if !ok {
		return nil
	}
	return m.runQuery(qp.inner.Value())
}

// fetchScriptsCmd scans m.scriptDir for .sql files and returns a ScriptsLoadedMsg.
func (m Modal) fetchScriptsCmd() tea.Cmd {
	dir := m.scriptDir
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ScriptsLoadedMsg{Dir: dir, Err: err}
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
				names = append(names, e.Name())
			}
		}
		return ScriptsLoadedMsg{Dir: dir, Names: names}
	}
}

// loadScriptCmd reads a script file and returns a ScriptLoadedMsg.
func loadScriptCmd(path, name string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		return ScriptLoadedMsg{Content: string(data), Name: name, Err: err}
	}
}

func (m Modal) runQuery(query string) tea.Cmd {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	sess, target := m.sess, m.target()
	return func() tea.Msg {
		rows, err := sess.Query(context.Background(), target, query)
		return QueryResultMsg{Rows: rows, Err: err}
	}
}
