package dbviewer

import (
	"simmer/internal/device"
	"simmer/internal/theme"

	"charm.land/lipgloss/v2"
)

// File returns the database the viewer is attached to.
func (m Modal) File() (dev device.Device, packageID, path, name string) {
	return m.device, m.packageID, m.dbPath, m.dbName
}

// QueryText returns the query editor's content.
func (m Modal) QueryText() string {
	if qp, ok := m.panes[paneQuery].(queryPaneAdapter); ok {
		return qp.inner.Value()
	}
	return ""
}

// SelectedTable returns the table under the explorer cursor — the table
// itself, or the table owning the column under it — or "".
func (m Modal) SelectedTable() string {
	ep, ok := m.panes[paneSidebar].(explorerPane)
	if !ok {
		return ""
	}
	node, parent := ep.inner.selectedNodeInfo()
	switch {
	case node == nil:
		return ""
	case node.kind == nodeKindTable:
		return node.realNameOrLabel()
	case node.kind == nodeKindCol:
		return parent
	}
	return ""
}

// RestoreQuery puts sql back in the editor without running it and without
// marking it as unsaved work.
func (m *Modal) RestoreQuery(sql string) {
	if qp, ok := m.panes[paneQuery].(queryPaneAdapter); ok {
		qp.inner.editor.SetValue(sql)
		qp.inner.editor.MoveToEnd()
		m.panes[paneQuery] = qp
	}
}

// RestoreTable moves the explorer cursor onto table once the table list
// arrives.
func (m *Modal) RestoreTable(table string) { m.pendingTable = table }

// SetLive tells the viewer whether this Simmer is the Active instance, for
// the indicator in its title row.
func (m *Modal) SetLive(live bool) { m.live = live }

// selectTable moves the explorer cursor onto the named table, if listed.
func (e *Explorer) selectTable(table string) bool {
	for i, it := range e.filteredVisibleItems() {
		if it.node.kind == nodeKindTable && it.node.realNameOrLabel() == table {
			e.cursor = i
			return true
		}
	}
	return false
}

// LiveBadgeHit reports whether the terminal cell (x, y) is on the
// Active-instance indicator at the right end of the title row.
func (m Modal) LiveBadgeHit(x, y int) bool {
	l := computeLayout(m.width, m.height)
	offX, offY := m.modalOffset(l)
	if y != offY+1 {
		return false
	}
	// The row is "│" + InnerW cells + "│"; the badge ends one blank before
	// the right border.
	end := offX + 1 + l.InnerW - 1
	start := end - lipgloss.Width(theme.LiveBadge(m.live))
	return x >= start && x < end
}
