package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Update handles tab-switch keys plus, while on the Files tab, tree
// navigation: up/down/j/k move the cursor and enter toggles expansion of the
// selected directory.
func (m MainPane) Update(msg tea.Msg) (MainPane, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	// While filter input is active, capture all keys before tab-switch handling.
	if m.tab == TabApps && m.appsFiltering {
		switch k.String() {
		case "esc", "enter":
			m.appsFiltering = false
		case "backspace":
			if len(m.appsFilter) > 0 {
				runes := []rune(m.appsFilter)
				m.appsFilter = string(runes[:len(runes)-1])
				m.appsIdx = 0
			}
		default:
			if k.Text != "" {
				m.appsFilter += k.Text
				m.appsIdx = 0
			}
		}
		return m, nil
	}

	switch k.String() {
	case "1":
		m.tab = TabInfo
		return m, nil
	case "2":
		m.tab = TabApps
		if app := m.SelectedApp(); app != nil {
			a := *app
			return m, func() tea.Msg { return AppFocusedMsg{App: a} }
		}
		return m, nil
	case "3":
		m.tab = TabLogs
		return m, nil
	case "4":
		m.tab = TabFiles
		if m.active != nil && m.tree == nil {
			app := m.selectedApp
			return m, func() tea.Msg { return RequestFileTreeMsg{App: app} }
		}
		return m, nil
	}
	switch m.tab {
	case TabFiles:
		rows := m.flattenTree()
		switch k.String() {
		case "up", "k":
			if m.treeIdx > 0 {
				m.treeIdx--
			}
		case "down", "j":
			if m.treeIdx < len(rows)-1 {
				m.treeIdx++
			}
		case "home", "g":
			m.treeIdx = 0
		case "end", "G":
			if len(rows) > 0 {
				m.treeIdx = len(rows) - 1
			}
		case "enter":
			if m.treeIdx < 0 || m.treeIdx >= len(rows) {
				return m, nil
			}
			n := rows[m.treeIdx].node
			if !n.IsDir {
				if m.active != nil {
					name := strings.ToLower(n.Name)
					if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".sqlite") || strings.HasSuffix(name, ".sqlite3") {
						dev := *m.active
						packageID := ""
						if m.selectedApp != nil {
							packageID = m.selectedApp.BundleID
						}
						nodeName := n.Name
						nodePath := n.Path
						return m, func() tea.Msg {
							return ShowSQLiteViewerMsg{
								Device:    dev,
								PackageID: packageID,
								DBPath:    nodePath,
								DBName:    nodeName,
							}
						}
					}
				}
				return m, nil
			}
			m.expanded[n.Path] = !m.expanded[n.Path]
			rows = m.flattenTree()
			if m.treeIdx >= len(rows) {
				m.treeIdx = len(rows) - 1
			}
			if m.treeIdx < 0 {
				m.treeIdx = 0
			}
		}
	case TabApps:
		filtered := m.filteredApps()
		prev := m.appsIdx
		switch k.String() {
		case "/":
			m.appsFiltering = true
		case "esc":
			if m.appsFilter != "" {
				m.appsFilter = ""
				m.appsIdx = 0
				return m, nil
			}
		case "up", "k":
			if m.appsIdx > 0 {
				m.appsIdx--
			}
		case "down", "j":
			if m.appsIdx < len(filtered)-1 {
				m.appsIdx++
			}
		case "home", "g":
			m.appsIdx = 0
		case "end", "G":
			if len(filtered) > 0 {
				m.appsIdx = len(filtered) - 1
			}
		case "space":
			if len(filtered) == 0 || m.appsIdx >= len(filtered) {
				return m, nil
			}
			app := filtered[m.appsIdx]
			if m.selectedApp != nil && m.selectedApp.BundleID == app.BundleID {
				m.selectedApp = nil
				m.tree = nil
				return m, func() tea.Msg { return StopLogStreamMsg{} }
			}
			a := app
			m.selectedApp = &a
			m.tree = nil
			return m, func() tea.Msg { return RequestLogStreamMsg{App: a} }
		}
		if m.appsIdx != prev {
			if app := m.SelectedApp(); app != nil {
				a := *app
				return m, func() tea.Msg { return AppFocusedMsg{App: a} }
			}
		}
	case TabLogs:
		var cmd tea.Cmd
		m.logsVP, cmd = m.logsVP.Update(msg)
		return m, cmd
	case TabInfo:
		n := len(m.info.Fields)
		switch k.String() {
		case "up", "k":
			if m.infoIdx > 0 {
				m.infoIdx--
			}
		case "down", "j":
			if m.infoIdx < n-1 {
				m.infoIdx++
			}
		case "home", "g":
			m.infoIdx = 0
		case "end", "G":
			if n > 0 {
				m.infoIdx = n - 1
			}
		case "space":
			if m.infoIdx < 0 || m.infoIdx >= n {
				return m, nil
			}
			return m, CopyToClipboardCmd(m.info.Fields[m.infoIdx].Value)
		}
	}
	return m, nil
}
