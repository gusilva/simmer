package dbviewer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"simmer/internal/device"
	"simmer/internal/ui/uitest"

	tea "charm.land/bubbletea/v2"
)

type closedMsg struct{}

// newTestModalInEditor returns a sized Modal with an empty query editor that
// has focus, vim mode as given, and whose onClose reports closedMsg.
func newTestModalInEditor(t *testing.T, vim bool) Modal {
	t.Helper()
	m := newTestModalWithTables()
	m.onClose = func() tea.Msg { return closedMsg{} }
	qp := m.panes[paneQuery].(queryPaneAdapter)
	m.panes[paneQuery] = queryPaneAdapter{qp.inner.withVimMode(vim).withScriptDir(t.TempDir())}
	m = m.setQueryText("")
	m, _ = m.setFocus(paneQuery)
	return m
}

func keyOf(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "ctrl+h":
		return tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl}
	case "ctrl+space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Mod: tea.ModCtrl}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

// typeKeys presses each key in turn, collecting the commands they return.
// Each character of a plain word is one key; named keys are their own entry.
func typeKeys(m Modal, keys ...string) (Modal, []tea.Cmd) {
	var cmds []tea.Cmd
	for _, k := range keys {
		var presses []tea.KeyPressMsg
		if len([]rune(k)) > 1 && keyOf(k).Text == "" {
			presses = []tea.KeyPressMsg{keyOf(k)}
		} else {
			for _, r := range k {
				presses = append(presses, keyOf(string(r)))
			}
		}
		for _, p := range presses {
			var cmd tea.Cmd
			m, cmd = m.Update(p)
			cmds = append(cmds, cmd)
		}
	}
	return m, cmds
}

// settle runs cmds as the Bubble Tea runtime would and reports whether one
// closed the viewer.
func settle(m Modal, cmds []tea.Cmd) (Modal, bool) {
	m, closes := uitest.Settle(m, cmds, func(msg tea.Msg) bool {
		_, ok := msg.(closedMsg)
		return ok
	})
	return m, len(closes) > 0
}

func editorText(m Modal) string { return m.panes[paneQuery].(queryPaneAdapter).inner.Value() }

func TestModal_PlainEditor(t *testing.T) {
	t.Run("q is text", func(t *testing.T) {
		m, cmds := typeKeys(newTestModalInEditor(t, false), "q")
		if _, closed := settle(m, cmds); closed {
			t.Fatal("typing q closed the viewer")
		}
		if got := editorText(m); got != "q" {
			t.Errorf("editor = %q, want %q", got, "q")
		}
	})
	t.Run("esc moves to the explorer", func(t *testing.T) {
		m, cmds := typeKeys(newTestModalInEditor(t, false), "esc")
		if _, closed := settle(m, cmds); closed {
			t.Fatal("esc in the editor closed the viewer")
		}
		if m.focus != paneSidebar {
			t.Errorf("focus = %d, want explorer (%d)", m.focus, paneSidebar)
		}
		// From the explorer, esc closes as before.
		if _, closed := settle(typeKeys(m, "esc")); !closed {
			t.Error("esc outside the editor did not close the viewer")
		}
	})
}

func TestModal_VimEditor(t *testing.T) {
	t.Run("insert, then esc stays in the editor", func(t *testing.T) {
		m, cmds := typeKeys(newTestModalInEditor(t, true), "i", "q", "?", "esc")
		if _, closed := settle(m, cmds); closed {
			t.Fatal("the viewer closed")
		}
		if got := editorText(m); got != "q?" {
			t.Errorf("editor = %q, want %q", got, "q?")
		}
		if m.focus != paneQuery {
			t.Errorf("focus = %d, want query (%d)", m.focus, paneQuery)
		}
		if got := m.panes[paneQuery].(queryPaneAdapter).inner.modeLabel(); got != "NORMAL" {
			t.Errorf("mode = %q, want NORMAL", got)
		}
	})
	t.Run("esc in normal mode moves to the explorer", func(t *testing.T) {
		m, _ := typeKeys(newTestModalInEditor(t, true), "i", "x", "esc", "esc")
		if m.focus != paneSidebar {
			t.Errorf("focus = %d, want explorer (%d)", m.focus, paneSidebar)
		}
		if got := editorText(m); got != "x" {
			t.Errorf("editor = %q, want %q", got, "x")
		}
	})
	t.Run("esc cancels a half-typed command on the way out", func(t *testing.T) {
		m := newTestModalInEditor(t, true)
		m = m.setQueryText("one\ntwo\nthree")
		m, _ = typeKeys(m, "g", "g", "d", "esc")
		if m.focus != paneSidebar {
			t.Fatalf("focus = %d, want explorer", m.focus)
		}
		m, _ = m.setFocus(paneQuery)
		m, _ = typeKeys(m, "j")
		if got := editorText(m); got != "one\ntwo\nthree" {
			t.Errorf("a pending d completed on return: editor = %q", got)
		}
	})
	t.Run("esc in visual mode stays in the editor", func(t *testing.T) {
		m, _ := typeKeys(newTestModalInEditor(t, true), "v", "esc")
		if m.focus != paneQuery {
			t.Errorf("focus = %d, want query", m.focus)
		}
	})
	t.Run("tab belongs to the editor", func(t *testing.T) {
		m, _ := typeKeys(newTestModalInEditor(t, true), "i", "tab")
		if m.focus != paneQuery {
			t.Errorf("tab moved focus to %d", m.focus)
		}
	})
	t.Run("ctrl+h leaves the editor", func(t *testing.T) {
		m, _ := typeKeys(newTestModalInEditor(t, true), "i", "ctrl+h")
		if m.focus != paneSidebar {
			t.Errorf("focus = %d, want explorer (%d)", m.focus, paneSidebar)
		}
	})
	t.Run(":q closes the viewer", func(t *testing.T) {
		m, cmds := typeKeys(newTestModalInEditor(t, true), ":q", "enter")
		if _, closed := settle(m, cmds); !closed {
			t.Error(":q did not close the viewer")
		}
	})
	t.Run(":w name writes the Script", func(t *testing.T) {
		m := newTestModalInEditor(t, true)
		m, cmds := typeKeys(m, "i", "SELECT 1;", "esc", ":w q.sql", "enter")
		m, _ = settle(m, cmds)
		qp := m.panes[paneQuery].(queryPaneAdapter).inner
		data, err := os.ReadFile(filepath.Join(qp.scriptDir, "q.sql"))
		if err != nil {
			t.Fatalf("read saved Script: %v", err)
		}
		if string(data) != "SELECT 1;" {
			t.Errorf("saved %q, want %q", data, "SELECT 1;")
		}
		if qp.filePath != "q.sql" || qp.state != fileStateClean {
			t.Errorf("file = %q state = %v, want q.sql clean", qp.filePath, qp.state)
		}
	})
	t.Run("paste inserts verbatim in normal mode", func(t *testing.T) {
		m := newTestModalInEditor(t, true)
		m, _ = m.Update(tea.PasteMsg{Content: "SELECT *\n  FROM t;"})
		if got := editorText(m); got != "SELECT *\n  FROM t;" {
			t.Errorf("editor = %q", got)
		}
		if got := m.panes[paneQuery].(queryPaneAdapter).inner.modeLabel(); got != "NORMAL" {
			t.Errorf("mode after paste = %q, want NORMAL", got)
		}
	})
}

func TestModal_QOutsideEditorCloses(t *testing.T) {
	m := newTestModalWithTables()
	m.onClose = func() tea.Msg { return closedMsg{} }
	if _, closed := settle(typeKeys(m, "q")); !closed {
		t.Error("q in the explorer did not close the viewer")
	}
}

func TestModal_Completion(t *testing.T) {
	withSchema := func(m Modal) Modal {
		return m.withQuerySchema(newSQLSchema([]device.SchemaTable{
			{Name: "users", Columns: []string{"id", "name"}},
		}))
	}
	menu := func(m Modal) bool { return m.completionMenuVisible() }

	for _, vim := range []bool{true, false} {
		name := "plain"
		if vim {
			name = "vim"
		}
		t.Run(name+": ctrl+space opens, tab accepts", func(t *testing.T) {
			m := withSchema(newTestModalInEditor(t, vim))
			keys := []string{"SELECT * FROM us", "ctrl+space"}
			if vim {
				keys = append([]string{"i"}, keys...)
			}
			m, cmds := typeKeys(m, keys...)
			m, _ = settle(m, cmds)
			if !menu(m) {
				t.Fatal("completion menu did not open")
			}
			m, _ = typeKeys(m, "tab")
			if got := editorText(m); got != "SELECT * FROM users" {
				t.Errorf("editor = %q", got)
			}
			if menu(m) {
				t.Error("menu still open after accepting")
			}
		})
	}

	t.Run("vim: the key entering insert mode opens no menu", func(t *testing.T) {
		m := withSchema(newTestModalInEditor(t, true))
		m = m.setQueryText("SELECT * FROM us")
		qp := m.panes[paneQuery].(queryPaneAdapter)
		qp.inner.editor.WithCompletionDebounce(time.Millisecond) // shorter than uitest.Quiet, so the tick fires in settle
		m.panes[paneQuery] = qp
		m, cmds := typeKeys(m, "A")
		m, _ = settle(m, cmds)
		if menu(m) {
			t.Error("pressing A opened the completion menu")
		}
	})

	t.Run("plain: esc closes the menu, not the editor", func(t *testing.T) {
		m := withSchema(newTestModalInEditor(t, false))
		m, cmds := typeKeys(m, "SELECT * FROM us", "ctrl+space")
		m, _ = settle(m, cmds)
		if !menu(m) {
			t.Fatal("completion menu did not open")
		}
		m, _ = typeKeys(m, "esc")
		if menu(m) || m.focus != paneQuery {
			t.Errorf("menu open = %v, focus = %d; want closed, still in the editor", menu(m), m.focus)
		}
		m, _ = typeKeys(m, "esc")
		if m.focus != paneSidebar {
			t.Errorf("second esc: focus = %d, want explorer", m.focus)
		}
	})

	t.Run("tables loaded give names, columns follow", func(t *testing.T) {
		m := newTestModalInEditor(t, true)
		m, _ = m.Update(TablesLoadedMsg{Objects: device.SQLiteObjects{
			Tables: []device.SchemaObject{{Name: "users", DisplayName: "People"}},
		}})
		qp := m.panes[paneQuery].(queryPaneAdapter).inner
		if tbl, ok := qp.schema.table("users"); !ok || len(tbl.Columns) != 0 {
			t.Fatalf("after tables: %+v %v, want users without columns", tbl, ok)
		}
		m, _ = m.Update(SchemaColumnsLoadedMsg{Tables: []device.SchemaTable{{Name: "users", Columns: []string{"id"}}}})
		qp = m.panes[paneQuery].(queryPaneAdapter).inner
		if tbl, _ := qp.schema.table("users"); len(tbl.Columns) != 1 {
			t.Errorf("after columns: %+v", tbl)
		}
		if got := qp.schema.label("users"); got != "People" {
			t.Errorf("label after columns = %q, want the tables query's People", got)
		}
		m, _ = m.Update(SchemaColumnsLoadedMsg{Err: errors.New("no such function: pragma_table_info")})
		qp = m.panes[paneQuery].(queryPaneAdapter).inner
		if _, ok := qp.schema.table("users"); !ok {
			t.Error("a failed column load dropped the names")
		}
	})
}
