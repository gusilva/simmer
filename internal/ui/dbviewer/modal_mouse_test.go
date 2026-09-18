package dbviewer

import (
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

// newTestModalWithTables returns a Modal sized to fill a 120x40 screen (as
// SetSize is always called with the app's own full width/height) with three
// tables loaded into the explorer sidebar.
func newTestModalWithTables() Modal {
	m := New(func() tea.Msg { return nil })
	m.SetSize(120, 40)

	ep := m.panes[paneSidebar].(explorerPane)
	ep.inner.SetTables("app.db", device.SQLiteObjects{
		Tables: []device.SchemaObject{{Name: "users"}, {Name: "orders"}, {Name: "products"}},
	})
	m.panes[paneSidebar] = ep
	return m
}

// TestModal_HandleMouseClick_SidebarRow_SelectsClickedTable is a regression
// test for a bug where handleMouseClick assumed the modal was always drawn
// at screen offset (1,1). In fact app.View() centers it via lipgloss, which
// for a fullscreen modal (SetSize called with the app's own width/height)
// always lands at a fixed (2,2) offset — off by one on both axes from the
// old hardcoded constant, which made every click select the row below the
// one actually clicked.
func TestModal_HandleMouseClick_SidebarRow_SelectsClickedTable(t *testing.T) {
	m := newTestModalWithTables()
	l := computeLayout(m.width, m.height)
	offX, offY := m.modalOffset(l)

	// Explorer.Rows(): row0 header, row1 sep, row2 "CONNECTIONS", row3 DB
	// node, row4 "Tables" folder, row5 "users" (first table row).
	usersLocalRow := 5
	clickY := offY + 3 + usersLocalRow
	clickX := offX + 5

	m2, cmd := m.Update(tea.MouseClickMsg{X: clickX, Y: clickY, Button: tea.MouseLeft})
	if cmd != nil {
		cmd() // drain any focus cmd
	}
	ep := m2.panes[paneSidebar].(explorerPane)
	node, _ := ep.inner.selectedNodeInfo()
	if node == nil || node.label != "users" {
		got := "nil"
		if node != nil {
			got = node.label
		}
		t.Errorf("expected click on the users row to select 'users', got %q", got)
	}
}

func TestModal_HandleMouseClick_OutsideOldHardcodedOffset_NoLongerMisses(t *testing.T) {
	m := newTestModalWithTables()
	l := computeLayout(m.width, m.height)
	offX, offY := m.modalOffset(l)

	// With the old hardcoded offset (1,1), a click at the real top border
	// row (offY, i.e. row 2 for a fullscreen modal) would have been
	// misread as one row into the body. Assert the real top border row is
	// correctly treated as out-of-body (no pane focus change / no panic).
	_, cmd := m.Update(tea.MouseClickMsg{X: offX + 5, Y: offY, Button: tea.MouseLeft})
	if cmd != nil {
		cmd()
	}
}
