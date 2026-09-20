package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestActionMenuModal_EnterSelectsHighlightedItem(t *testing.T) {
	ran := false
	m := NewActionMenuModal("Device A", []ActionMenuItem{
		{Label: "Boot", Cmd: func() tea.Msg { ran = true; return nil }},
		{Label: "Delete", Cmd: func() tea.Msg { return nil }},
	})

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Fatal("navigation must not emit a cmd")
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a cmd from enter")
	}
	msg, ok := cmd().(ActionMenuSelectedMsg)
	if !ok {
		t.Fatalf("expected ActionMenuSelectedMsg, got %#v", msg)
	}
	msg.Cmd()
	if ran {
		t.Fatal("expected the second item's (Delete) cmd to run, not the first's (Boot)")
	}
}

func TestActionMenuModal_EscCancels(t *testing.T) {
	m := NewActionMenuModal("Device A", []ActionMenuItem{{Label: "Boot", Cmd: func() tea.Msg { return nil }}})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a cmd from esc")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Fatal("expected CancelOverlayMsg from esc")
	}
}
