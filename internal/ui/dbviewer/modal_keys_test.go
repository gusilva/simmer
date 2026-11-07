package dbviewer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

// runCmd runs cmd and returns the messages it produces, expanding batches.
// A command still blocked after a short wait — the editor's signal listener
// when no signal is pending — contributes nothing.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(50 * time.Millisecond):
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runCmd(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

// settle feeds the messages cmds produce back into m until none are left,
// the way the Bubble Tea runtime would, and reports whether one closed it.
// Commands are only run here, after the keys: the editor's one signal
// listener is armed by the first key and must still be waiting when the
// signal arrives.
func settle(m Modal, cmds []tea.Cmd) (Modal, bool) {
	closed := false
	for range 100 {
		var msgs []tea.Msg
		for _, c := range cmds {
			msgs = append(msgs, runCmd(c)...)
		}
		if len(msgs) == 0 {
			break
		}
		cmds = nil
		for _, msg := range msgs {
			if _, ok := msg.(closedMsg); ok {
				closed = true
				continue
			}
			var cmd tea.Cmd
			m, cmd = m.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, closed
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
