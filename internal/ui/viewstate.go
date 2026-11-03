package ui

import "simmer/internal/device"

// The setters in this file put a component back where a previous Simmer left
// it (the View state). Each one reports whether its target still exists, so
// the caller can fall back to the nearest part that does.

// SelectDevice moves the cursor onto the device with id, in whichever box
// holds it, expanding a collapsed group and clearing a filter that hides it.
func (s *Sidebar) SelectDevice(id string) bool {
	for i, d := range s.booted {
		if d.ID != id {
			continue
		}
		switch d.Platform {
		case device.PlatformIOS:
			s.iosCollapsed = false
		case device.PlatformAndroid:
			s.androidCollapsed = false
		}
		for pi, p := range s.onlinePositions() {
			if !p.isHeader && p.bootedIdx == i {
				s.focused = PaneBooted
				s.bootedIdx = pi
				return true
			}
		}
	}
	for tab, devs := range map[offlineTab][]device.Device{tabIOS: s.virtIOS, tabAndroid: s.virtAnd} {
		for i, d := range devs {
			if d.ID == id {
				s.filterQuery = ""
				s.filterMode = false
				s.focused = PaneAvailable
				s.offlinePlatform = tab
				s.availIdx = i
				return true
			}
		}
	}
	return false
}

// ShowFilesPanel expands the Files panel. Like the "3" key, it asks for the
// tree only when none is loaded.
func (m *MainPane) ShowFilesPanel() *RequestFileTreeMsg {
	m.panel = panelFiles
	if m.active == nil || m.tree != nil {
		return nil
	}
	return &RequestFileTreeMsg{App: m.selectedApp}
}

// SelectAppByID moves the Apps cursor onto bundleID.
func (m *MainPane) SelectAppByID(bundleID string) bool {
	for i, a := range m.filteredApps() {
		if a.BundleID == bundleID {
			m.appsIdx = i
			return true
		}
	}
	return false
}

// PinAppByID pins bundleID for Files browsing, as space does, unless it is
// already pinned.
func (m *MainPane) PinAppByID(bundleID string) bool {
	if m.selectedApp != nil && m.selectedApp.BundleID == bundleID {
		return true
	}
	for _, a := range m.apps {
		if a.BundleID == bundleID {
			m.TogglePinnedApp(a)
			return true
		}
	}
	return false
}

// PinnedApp returns the app pinned for Files browsing, or nil.
func (m MainPane) PinnedApp() *device.App { return m.selectedApp }

// AppsLoaded reports whether the app list for the active device has arrived.
func (m MainPane) AppsLoaded() bool { return m.active != nil && !m.appsLoading }
