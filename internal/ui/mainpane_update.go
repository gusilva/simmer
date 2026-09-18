package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Update handles panel-toggle keys (2/3), the device-info key (i), the esc
// unwind chain, and — for whichever panel is expanded — its navigation and
// action keys.
func (m MainPane) Update(msg tea.Msg) (MainPane, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	// While filter input is active, capture all keys before anything else.
	if m.panel == panelApps && m.appsFiltering {
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
	case "2":
		m.panel = panelApps
		if app := m.SelectedApp(); app != nil {
			a := *app
			return m, func() tea.Msg { return AppFocusedMsg{App: a} }
		}
		return m, nil
	case "3":
		m.panel = panelFiles
		if m.active != nil && m.tree == nil {
			app := m.selectedApp
			return m, func() tea.Msg { return RequestFileTreeMsg{App: app} }
		}
		return m, nil
	case "i":
		if m.active == nil {
			return m, nil
		}
		dev := *m.active
		return m, func() tea.Msg { return ShowDeviceInfoMsg{Device: dev} }
	case "esc":
		// Unwind chain: clear filter → collapse Apps → release focus.
		if m.panel == panelApps && m.appsFilter != "" {
			m.appsFilter = ""
			m.appsIdx = 0
			return m, nil
		}
		if m.panel == panelApps {
			m.panel = panelFiles
			return m, nil
		}
		return m, func() tea.Msg { return ReleaseFocusMsg{} }
	}

	switch m.panel {
	case panelFiles:
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
	case panelApps:
		filtered := m.filteredApps()
		prev := m.appsIdx
		switch k.String() {
		case "/":
			m.appsFiltering = true
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
		case "a":
			if m.active == nil {
				return m, nil
			}
			dev := *m.active
			return m, func() tea.Msg { return ShowInstallAppMsg{Device: dev} }
		case "d":
			if len(filtered) == 0 || m.appsIdx >= len(filtered) || m.active == nil {
				return m, nil
			}
			app := filtered[m.appsIdx]
			if app.Type == "System" {
				return m, nil
			}
			dev := *m.active
			a := app
			return m, func() tea.Msg { return ShowDeleteAppMsg{Device: dev, App: a} }
		case "r":
			if len(filtered) == 0 || m.appsIdx >= len(filtered) || m.active == nil {
				return m, nil
			}
			dev := *m.active
			a := filtered[m.appsIdx]
			return m, func() tea.Msg { return RequestRebuildMsg{Device: dev, App: a} }
		case "enter":
			if len(filtered) == 0 || m.appsIdx >= len(filtered) || m.active == nil {
				return m, nil
			}
			dev := *m.active
			a := filtered[m.appsIdx]
			return m, func() tea.Msg { return LaunchAppMsg{Device: dev, App: a} }
		case "c":
			if len(filtered) == 0 || m.appsIdx >= len(filtered) || m.active == nil {
				return m, nil
			}
			dev := *m.active
			a := filtered[m.appsIdx]
			return m, func() tea.Msg { return ShowCloseAppMsg{Device: dev, App: a} }
		case "space":
			// Pin / unpin the app whose sandbox the Files panel browses.
			if len(filtered) == 0 || m.appsIdx >= len(filtered) {
				return m, nil
			}
			app := filtered[m.appsIdx]
			if m.selectedApp != nil && m.selectedApp.BundleID == app.BundleID {
				m.selectedApp = nil
				m.tree = nil
				return m, nil
			}
			a := app
			m.selectedApp = &a
			m.tree = nil
			return m, nil
		case "l":
			// Toggle writing the highlighted app's logs to a file.
			if len(filtered) == 0 || m.appsIdx >= len(filtered) {
				return m, nil
			}
			a := filtered[m.appsIdx]
			if m.loggingBundle == a.BundleID {
				return m, func() tea.Msg { return StopAppLoggingMsg{} }
			}
			return m, func() tea.Msg { return StartAppLoggingMsg{App: a} }
		}
		if m.appsIdx != prev {
			if app := m.SelectedApp(); app != nil {
				a := *app
				return m, func() tea.Msg { return AppFocusedMsg{App: a} }
			}
		}
	}
	return m, nil
}
