package app

import (
	"time"

	"simmer/internal/device"
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
)

// doubleClickThreshold is the max gap between two clicks on an
// already-selected row for the second one to open its action menu.
const doubleClickThreshold = 400 * time.Millisecond

// lastClickInfo tracks the previous click's target identity, for detecting a
// double-click on an already-selected row (see registerClick).
type lastClickInfo struct {
	id          string
	at          time.Time
	wasReselect bool // true if that click landed on the row already selected before it
}

// registerClick records a click against id (the row identity — device UDID,
// app bundle id, or file path — clicked after HandleClick ran) and preID
// (the identity that was selected immediately before this click). It reports
// whether this click completes a double-click: the row must already have
// been selected before the *first* of the two clicks, i.e. neither click in
// the pair may itself be the one that changed the selection.
func (m *model) registerClick(id, preID string) bool {
	now := time.Now()
	reselect := id != "" && id == preID
	isDouble := reselect && m.lastClick.wasReselect && id == m.lastClick.id &&
		now.Sub(m.lastClick.at) < doubleClickThreshold
	m.lastClick = lastClickInfo{id: id, at: now, wasReselect: reselect}
	return isDouble
}

func deviceIdentity(d *device.Device) string {
	if d == nil {
		return ""
	}
	return "device:" + d.ID
}

// mainPaneSelectionID returns the identity of whichever row is currently
// selected in the main pane's active panel (Apps or Files).
func (m model) mainPaneSelectionID() string {
	if m.mainPane.IsFilesPanel() {
		if n := m.mainPane.SelectedTreeNode(); n != nil {
			return "file:" + n.Path
		}
		return ""
	}
	if a := m.mainPane.SelectedApp(); a != nil {
		return "app:" + a.BundleID
	}
	return ""
}

// buildDeviceActionMenu returns the double-click action menu for a sidebar
// device row. Each item's Cmd is exactly what the equivalent keyboard
// shortcut already runs.
func (m model) buildDeviceActionMenu(dev device.Device) *ui.ActionMenuModal {
	var items []ui.ActionMenuItem

	switch {
	case dev.Status == device.StatusRunning:
		d := dev
		items = append(items, ui.ActionMenuItem{
			Label: "Load Device",
			Cmd:   func() tea.Msg { return ui.LoadDeviceMsg{Device: d} },
		})
		if dev.Kind != device.KindPhysical {
			items = append(items, ui.ActionMenuItem{
				Label: "Shutdown",
				Cmd:   m.shutdownDeviceCmd(d),
			})
		}
	default:
		d := dev
		items = append(items, ui.ActionMenuItem{
			Label: "Boot",
			Cmd:   func() tea.Msg { return ui.StartBootMsg{Device: d} },
		})
		items = append(items, ui.ActionMenuItem{
			Label: "Delete",
			Cmd:   func() tea.Msg { return ui.ShowDeleteSimulatorMsg{Device: d} },
		})
	}

	items = append(items,
		ui.ActionMenuItem{
			Label: "Add Device…",
			Cmd:   func() tea.Msg { return ui.ShowPlatformPickerMsg{} },
		},
		ui.ActionMenuItem{
			Label: "Refresh",
			Cmd:   func() tea.Msg { return ui.RefreshDevicesMsg{} },
		},
	)

	menu := ui.NewActionMenuModal(dev.Name, items)
	return &menu
}

// buildAppActionMenu returns the double-click action menu for an Apps-panel
// row.
func (m model) buildAppActionMenu(dev device.Device, app device.App) *ui.ActionMenuModal {
	d, a := dev, app
	var items []ui.ActionMenuItem

	items = append(items,
		ui.ActionMenuItem{
			Label: "Launch",
			Cmd:   func() tea.Msg { return ui.LaunchAppMsg{Device: d, App: a} },
		},
		ui.ActionMenuItem{
			Label: "Close",
			Cmd:   func() tea.Msg { return ui.ShowCloseAppMsg{Device: d, App: a} },
		},
		ui.ActionMenuItem{
			Label: "Browse Files",
			Cmd:   func() tea.Msg { return ui.PinAppMsg{App: a} },
		},
	)

	if m.mainPane.LoggingBundle() == a.BundleID {
		items = append(items, ui.ActionMenuItem{
			Label: "Stop Logging to File",
			Cmd:   func() tea.Msg { return ui.StopAppLoggingMsg{} },
		})
	} else {
		items = append(items, ui.ActionMenuItem{
			Label: "Log to File",
			Cmd:   func() tea.Msg { return ui.StartAppLoggingMsg{App: a} },
		})
	}

	if dev.Kind == device.KindVirtual && a.IsLocal() {
		items = append(items, ui.ActionMenuItem{
			Label: "Rebuild & Reinstall",
			Cmd:   func() tea.Msg { return ui.RequestRebuildMsg{Device: d, App: a} },
		})
	}

	if a.Type != "System" {
		items = append(items, ui.ActionMenuItem{
			Label: "Delete App",
			Cmd:   func() tea.Msg { return ui.ShowDeleteAppMsg{Device: d, App: a} },
		})
	}

	items = append(items, ui.ActionMenuItem{
		Label: "Install App…",
		Cmd:   func() tea.Msg { return ui.ShowInstallAppMsg{Device: d} },
	})

	menu := ui.NewActionMenuModal(a.Label(), items)
	return &menu
}

// buildFileActionMenu returns the double-click action menu for a Files-panel
// row.
func (m model) buildFileActionMenu(dev device.Device, node device.FileNode) *ui.ActionMenuModal {
	var items []ui.ActionMenuItem

	switch {
	case node.IsDir:
		items = append(items, ui.ActionMenuItem{
			Label: "Expand/Collapse",
			Cmd:   func() tea.Msg { return ui.ActivateTreeRowMsg{} },
		})
	case ui.IsDatabaseFile(node.Name):
		items = append(items, ui.ActionMenuItem{
			Label: "Open Database",
			Cmd:   func() tea.Msg { return ui.ActivateTreeRowMsg{} },
		})
	}

	items = append(items, ui.ActionMenuItem{
		Label: "Device Info",
		Cmd:   func() tea.Msg { return ui.ShowDeviceInfoMsg{Device: dev} },
	})

	menu := ui.NewActionMenuModal(node.Name, items)
	return &menu
}
