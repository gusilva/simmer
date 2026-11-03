package dbviewer

import (
	"strings"
	"testing"

	"simmer/internal/config"
	"simmer/internal/device"
	"simmer/internal/session"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRestoreTable_SelectsItOnceListed(t *testing.T) {
	m := New(func() tea.Msg { return nil }, session.New(nil, nil, config.Default()))
	m.RestoreTable("orders")
	m, _ = m.Update(TablesLoadedMsg{Objects: device.SQLiteObjects{
		Tables: []device.SchemaObject{{Name: "users"}, {Name: "orders"}},
	}})
	if got := m.SelectedTable(); got != "orders" {
		t.Fatalf("selected %q, want orders", got)
	}
}

func TestRestoreQuery_NotRun(t *testing.T) {
	m := New(func() tea.Msg { return nil }, session.New(nil, nil, config.Default()))
	m.RestoreQuery("select * from users;")
	if got := m.QueryText(); got != "select * from users;" {
		t.Fatalf("query %q", got)
	}
	if m.lastStats.hasData {
		t.Fatal("restoring a query must not run it")
	}
}

func TestTitleRow_LiveBadge(t *testing.T) {
	m := New(func() tea.Msg { return nil }, session.New(nil, nil, config.Default()))
	m.SetSize(140, 40)
	if !strings.Contains(ansi.Strip(m.View()), "● live") {
		t.Fatal("live viewer should show the live badge")
	}
	m.SetLive(false)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "not live · ^T take over") {
		t.Fatal("superseded viewer should offer to take over")
	}

	// The hit area is on the title row, ending just before the border.
	title := strings.Split(view, "\n")[1]
	col := strings.Index(title, "○")
	x := len([]rune(title[:col])) + (140-computeLayout(140, 40).ModalW)/2
	if !m.LiveBadgeHit(x, 1+(40-computeLayout(140, 40).ModalH)/2) {
		t.Fatal("badge click not detected")
	}
}

// The title row must never be wider than the modal, however long the
// database name: an overflow pushes the right border out of line.
func TestTitleRow_FitsWithLongNames(t *testing.T) {
	name := "john.doe@fo-gustavo.com#BusinessObject.db"
	path := "/data/user/0/com.osapiens.operations.maintenanceos.qa/databases/" + name
	for _, live := range []bool{true, false} {
		for w := 40; w <= 220; w += 7 {
			m := New(func() tea.Msg { return nil }, session.New(nil, nil, config.Default()))
			m.SetSize(w, 40)
			m.dbName, m.dbPath, m.sqliteVersion = name, path, "3.44.3"
			m.SetLive(live)
			want := computeLayout(w, 40).ModalW
			for i, line := range strings.Split(m.View(), "\n") {
				// Below 80 columns the TUI shows its resize prompt instead of
				// the viewer (app.minTermWidth); only the title row is held
				// to the line there.
				if w < 80 && i != 1 {
					continue
				}
				if got := ansi.StringWidth(line); got != want {
					t.Fatalf("live=%v width=%d line %d is %d cells, want %d:\n%s", live, w, i, got, want, ansi.Strip(line))
				}
			}
		}
	}
}
