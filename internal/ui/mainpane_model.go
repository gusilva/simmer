package ui

import (
	"simmer/internal/device"

	"charm.land/bubbles/v2/help"
)

// mainPanel identifies which of the two stacked panels is expanded. The other
// is drawn as a one-line strip.
type mainPanel int

const (
	// panelApps expands the installed-apps list (default on device load — apps
	// load eagerly, the file tree does not).
	panelApps mainPanel = iota
	// panelFiles expands the filesystem tree + preview.
	panelFiles
)

// MainPane is the right-hand panel showing details for the active device.
// It is a pure UI component: callers pass in the active device and a loaded
// filesystem tree via SetDevice.
type MainPane struct {
	active        *device.Device    // Which device is selected (nil = none)
	panel         mainPanel         // Which panel is expanded: Files (default) or Apps
	tree          *device.FileNode  // Filesystem tree for the Files panel
	expanded      map[string]bool   // Which dirs are open in the tree
	treeIdx       int               // Cursor row in the tree
	apps          []device.App      // List of installed apps
	appsIdx       int               // Cursor row in the filtered apps list
	appsFilter    string            // Active fuzzy filter for apps
	appsFiltering bool              // Whether filter input is active
	selectedApp   *device.App       // App pinned (via space) for Files sandbox browsing
	info          device.DeviceInfo // Device info fields (rendered by the info overlay)
	infoOpen      bool              // Whether the device-info overlay is open (hides the header hint)
	loggingBundle string            // Bundle id whose logs are currently written to file ("" = none)
	focused       bool              // Does this pane have keyboard focus?
	width         int               // Outer width available to the pane (including borders)
	height        int               // Outer height available to the pane (including borders)
}

// NewMainPane returns an empty main pane.
func NewMainPane() MainPane {
	return MainPane{expanded: map[string]bool{}}
}

// SetSize sets the outer width/height available to the pane.
func (m *MainPane) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused marks whether the main pane holds the app's outer focus.
// Affects only border styling; key handling is owned by the parent.
func (m *MainPane) SetFocused(f bool) { m.focused = f }

// SetDevice loads a device and its filesystem root into the pane. Pass nil
// for both to clear the pane. Resets the apps list as well.
func (m *MainPane) SetDevice(d *device.Device, root *device.FileNode) {
	m.active = d
	m.panel = panelApps
	m.tree = root
	m.expanded = map[string]bool{}
	m.treeIdx = 0
	m.apps = nil
	m.appsIdx = 0
	m.appsFilter = ""
	m.appsFiltering = false
	m.selectedApp = nil
	m.info = device.DeviceInfo{}
	m.infoOpen = false
	m.loggingBundle = ""

	if root != nil {
		m.expanded[root.Path] = true

		for _, c := range root.Children {
			if c.IsDir {
				m.expanded[c.Path] = true
			}
		}
	}
}

// SyncActiveDevice updates the active device's fields from the refreshed list.
// All other pane state (tabs, tree, apps, info, logs) is preserved.
// If the active device is no longer in the list (e.g. deleted), the pane is cleared.
func (m *MainPane) SyncActiveDevice(devs []device.Device) {
	if m.active == nil {
		return
	}
	for _, d := range devs {
		if d.ID == m.active.ID {
			*m.active = d
			for i := range m.info.Fields {
				if m.info.Fields[i].Key == "Status" {
					m.info.Fields[i].Value = string(d.Status)
					break
				}
			}
			return
		}
	}
	m.active = nil
}

// SetTree replaces only the filesystem tree without resetting the rest of the
// pane state. Use this when the tree loads asynchronously after SetDevice.
func (m *MainPane) SetTree(root *device.FileNode) {
	m.tree = root
	m.expanded = map[string]bool{}
	m.treeIdx = 0
	if root != nil {
		m.expanded[root.Path] = true
		for _, c := range root.Children {
			if c.IsDir {
				m.expanded[c.Path] = true
			}
		}
	}
}

// SetApps replaces the list of installed apps shown in the Apps tab.
func (m *MainPane) SetApps(apps []device.App) {
	m.apps = apps
	m.appsFilter = ""
	m.appsFiltering = false
	m.appsIdx = 0
}

// filteredApps returns apps matching the active filter, or all apps when empty.
func (m MainPane) filteredApps() []device.App {
	if m.appsFilter == "" {
		return m.apps
	}
	out := make([]device.App, 0, len(m.apps))
	for _, app := range m.apps {
		if fuzzyMatch(m.appsFilter, app.Label()) || fuzzyMatch(m.appsFilter, app.BundleID) {
			out = append(out, app)
		}
	}
	return out
}

// SetInfo replaces the device info rendered by the info overlay.
func (m *MainPane) SetInfo(info device.DeviceInfo) { m.info = info }

// DeviceInfo returns the loaded device info (may have zero fields while loading).
func (m MainPane) DeviceInfo() device.DeviceInfo { return m.info }

// ActiveDevice returns the currently loaded device, or nil.
func (m MainPane) ActiveDevice() *device.Device { return m.active }

// SetInfoOpen tells the pane whether the device-info overlay is showing, so the
// header can drop its "[i] more info" hint while the sheet is up.
func (m *MainPane) SetInfoOpen(open bool) { m.infoOpen = open }

// SelectedApp returns the app currently highlighted in the Apps tab, or nil.
func (m MainPane) SelectedApp() *device.App {
	filtered := m.filteredApps()
	if m.appsIdx < 0 || m.appsIdx >= len(filtered) {
		return nil
	}
	a := filtered[m.appsIdx]
	return &a
}

// IsDatabaseFile reports whether name looks like a SQLite database, by
// extension — the same rule the Files panel uses to offer its SQLite
// viewer.
func IsDatabaseFile(name string) bool { return isDatabaseFile(name) }

// IsFilesPanel reports whether the Files panel is currently expanded (as
// opposed to Apps).
func (m MainPane) IsFilesPanel() bool { return m.panel == panelFiles }

// SelectedTreeNode returns the file/dir node currently highlighted in the
// Files tab, or nil.
func (m MainPane) SelectedTreeNode() *device.FileNode {
	rows := m.flattenTree()
	if m.treeIdx < 0 || m.treeIdx >= len(rows) {
		return nil
	}
	return rows[m.treeIdx].node
}

// TogglePinnedApp pins app for the Files panel's sandbox browsing, or unpins
// it if it's already pinned. Shared by the "space" key and the double-click
// action menu.
func (m *MainPane) TogglePinnedApp(app device.App) {
	if m.selectedApp != nil && m.selectedApp.BundleID == app.BundleID {
		m.selectedApp = nil
		m.tree = nil
		return
	}
	a := app
	m.selectedApp = &a
	m.tree = nil
}

// SetLoggingBundle records which app's logs are currently being written to a
// file, so the Apps tab can badge that row. Pass "" to clear.
func (m *MainPane) SetLoggingBundle(bundleID string) { m.loggingBundle = bundleID }

// LoggingBundle returns the bundle id whose logs are currently written to
// file, or "" if none.
func (m MainPane) LoggingBundle() string { return m.loggingBundle }

// StartAppLoggingMsg is dispatched when the user presses "l" on an app row.
// The parent program starts streaming that app's logs to a file.
type StartAppLoggingMsg struct {
	App device.App
}

// StopAppLoggingMsg is dispatched when the user presses "l" again on the app
// currently being logged.
type StopAppLoggingMsg struct{}

// RequestFileTreeMsg is dispatched when the Files tab is opened. App is nil
// when no app is selected, in which case the parent should load the root tree.
type RequestFileTreeMsg struct {
	App *device.App
}

// AppFocusedMsg is dispatched whenever the cursor lands on an app row in the
// Apps panel. Useful for surfacing the selection in a status bar.
type AppFocusedMsg struct {
	App device.App
}

// ShowDeviceInfoMsg is dispatched when "i" is pressed in the main pane. The
// parent opens the device-info overlay.
type ShowDeviceInfoMsg struct {
	Device device.Device
}

// ReleaseFocusMsg is dispatched when "esc" in the main pane has nothing left to
// unwind. The parent hands keyboard focus back to the sidebar.
type ReleaseFocusMsg struct{}

// HasDevice reports whether a device is currently loaded.
func (m MainPane) HasDevice() bool { return m.active != nil }

// HelpKeyMap returns the title and key map for the "?" overlay, scoped to
// whichever panel is currently expanded.
func (m MainPane) HelpKeyMap() (string, help.KeyMap) {
	if m.panel == panelApps {
		return "Apps", MainPaneAppsKeys
	}
	return "Files", MainPaneFilesKeys
}
