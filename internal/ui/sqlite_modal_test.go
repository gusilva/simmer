package ui

import (
	"fmt"
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

func newTestSQLiteModal() SQLiteModal {
	dev := device.Device{Name: "iPhone 15", Platform: device.PlatformIOS}
	m, _ := NewSQLiteModal(dev, "com.example", "/path/to/db", "test.db")
	return m
}

// ── NewSQLiteModal ────────────────────────────────────────────────────────

func TestNewSQLiteModal_InitialState(t *testing.T) {
	m := newTestSQLiteModal()
	if !m.loading {
		t.Error("expected loading=true initially")
	}
	if m.mode != sqliteModeEdit {
		t.Error("expected sqliteModeEdit initially")
	}
	if m.dbName != "test.db" {
		t.Errorf("dbName: got %q", m.dbName)
	}
}

// ── SetSize ───────────────────────────────────────────────────────────────

func TestSQLiteModal_SetSize(t *testing.T) {
	m := newTestSQLiteModal()
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("SetSize: got %dx%d", m.width, m.height)
	}
}

// ── Update: non-key no-op ─────────────────────────────────────────────────

func TestSQLiteModal_Update_NonKey_NoOp(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	_, cmd := m.Update("not a key")
	if cmd != nil {
		t.Error("expected no cmd for non-key message in nav mode")
	}
}

// ── Update: sqliteModeEdit ────────────────────────────────────────────────

func TestSQLiteModal_Update_Edit_Esc_SwitchesToNav(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeEdit
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m2.mode != sqliteModeNav {
		t.Errorf("expected sqliteModeNav, got %v", m2.mode)
	}
}

func TestSQLiteModal_Update_Edit_Enter_RunsQuery(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeEdit
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter in edit mode")
	}
}

// ── Update: sqliteModeNav ─────────────────────────────────────────────────

func TestSQLiteModal_Update_Nav_Esc_Cancels(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc in nav mode")
	}
	if _, ok := cmd().(CancelOverlayMsg); !ok {
		t.Error("expected CancelOverlayMsg")
	}
}

func TestSQLiteModal_Update_Nav_Esc_ClearsSearch(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	m.searchQuery = "foo"
	m2, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Error("expected no cmd when clearing search")
	}
	if m2.searchQuery != "" {
		t.Error("expected searchQuery cleared")
	}
}

func TestSQLiteModal_Update_Nav_I_SwitchesToEdit(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	m2, _ := m.Update(tea.KeyPressMsg{Text: "i"})
	if m2.mode != sqliteModeEdit {
		t.Errorf("expected sqliteModeEdit, got %v", m2.mode)
	}
}

func TestSQLiteModal_Update_Nav_Slash_SwitchesToSearch(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	m2, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	if m2.mode != sqliteModeSearch {
		t.Errorf("expected sqliteModeSearch, got %v", m2.mode)
	}
}

func TestSQLiteModal_Update_Nav_Space_CopiesRow(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	m.rows = [][]string{{"name"}, {"users"}}
	m.SetSize(120, 40)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if cmd == nil {
		t.Fatal("expected clipboard cmd from space")
	}
}

func TestSQLiteModal_Update_Nav_LeftRight_Scroll(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	m.rows = [][]string{{"name"}, {"users"}}
	m.SetSize(120, 40)
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m3, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m3.colOffset < 0 {
		t.Error("colOffset must not go negative")
	}
}

func TestSQLiteModal_Update_Nav_Enter_RunsQuery(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter in nav mode")
	}
}

func TestSQLiteModal_Update_Nav_DownUp(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeNav
	m.rows = [][]string{{"name"}, {"table1"}, {"table2"}}
	m.SetSize(120, 40)
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m2.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m2.cursor)
	}
	m3, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m3.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m3.cursor)
	}
}

// ── Update: sqliteModeSearch ──────────────────────────────────────────────

func TestSQLiteModal_Update_Search_TextAppends(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeSearch
	m2, _ := m.Update(tea.KeyPressMsg{Text: "f"})
	if m2.searchQuery != "f" {
		t.Errorf("expected searchQuery 'f', got %q", m2.searchQuery)
	}
}

func TestSQLiteModal_Update_Search_Backspace(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeSearch
	m.searchQuery = "fo"
	m.rows = [][]string{{"name"}}
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m2.searchQuery != "f" {
		t.Errorf("expected 'f', got %q", m2.searchQuery)
	}
}

func TestSQLiteModal_Update_Search_Esc_ExitsSearch(t *testing.T) {
	m := newTestSQLiteModal()
	m.mode = sqliteModeSearch
	m2, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m2.mode != sqliteModeNav {
		t.Errorf("expected sqliteModeNav, got %v", m2.mode)
	}
}

// ── View ──────────────────────────────────────────────────────────────────

func TestSQLiteModal_View_Loading(t *testing.T) {
	m := newTestSQLiteModal()
	m.SetSize(120, 40)
	got := m.View()
	if !strings.Contains(got, "test.db") {
		t.Error("expected db name in loading view")
	}
}

func TestSQLiteModal_View_WithResults(t *testing.T) {
	m := newTestSQLiteModal()
	m.SetSize(120, 40)
	m.SetResult([][]string{{"name"}, {"users"}, {"orders"}}, nil)
	got := m.View()
	if got == "" {
		t.Error("expected non-empty view with results")
	}
}

func TestSQLiteModal_View_WithError(t *testing.T) {
	m := newTestSQLiteModal()
	m.SetSize(120, 40)
	m.SetResult(nil, fmt.Errorf("no such table"))
	got := m.View()
	if !strings.Contains(got, "no such table") {
		t.Error("expected error in view")
	}
}
