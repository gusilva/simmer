package dbviewer

import (
	"simmer/internal/config"
	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

// SettingsSavedMsg is returned after a config save attempt completes.
type SettingsSavedMsg struct{ Err error }

// SettingsCancelMsg is returned when the user dismisses the settings form.
type SettingsCancelMsg struct{}

func saveConfigCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := config.Save(cfg)
		return SettingsSavedMsg{Err: err}
	}
}

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

// QueryResultMsg carries the result of an executed SQL query.
type QueryResultMsg struct {
	Rows [][]string
	Err  error
}
