package dbviewer

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type closedMsg struct{}

// newTestModalInEditor returns a sized Modal with an empty query editor that
// has focus, and whose onClose reports closedMsg.
func newTestModalInEditor(t *testing.T) Modal {
	t.Helper()
	m := newTestModalWithTables()
	m.onClose = func() tea.Msg { return closedMsg{} }
	m = m.setQueryText("")
	m, _ = m.setFocus(paneQuery)
	return m
}

func press(m Modal, k tea.KeyPressMsg) (Modal, tea.Cmd) {
	next, cmd := m.Update(k)
	return next, cmd
}

func closes(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(closedMsg)
	return ok
}

func TestModal_QTypedInEditorIsText(t *testing.T) {
	m := newTestModalInEditor(t)
	m, cmd := press(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if closes(cmd) {
		t.Fatal("typing q in the editor closed the viewer")
	}
	if got := m.panes[paneQuery].(queryPaneAdapter).inner.Value(); got != "q" {
		t.Errorf("editor = %q, want %q", got, "q")
	}
}

func TestModal_EscInEditorMovesToExplorer(t *testing.T) {
	m := newTestModalInEditor(t)
	m, cmd := press(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if closes(cmd) {
		t.Fatal("esc in the editor closed the viewer")
	}
	if m.focus != paneSidebar {
		t.Errorf("focus = %d, want explorer (%d)", m.focus, paneSidebar)
	}

	// From the explorer, esc closes as before.
	if _, cmd = press(m, tea.KeyPressMsg{Code: tea.KeyEscape}); !closes(cmd) {
		t.Error("esc outside the editor did not close the viewer")
	}
}

func TestModal_QOutsideEditorCloses(t *testing.T) {
	m := newTestModalWithTables()
	m.onClose = func() tea.Msg { return closedMsg{} }
	if _, cmd := press(m, tea.KeyPressMsg{Code: 'q', Text: "q"}); !closes(cmd) {
		t.Error("q in the explorer did not close the viewer")
	}
}
