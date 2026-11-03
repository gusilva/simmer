package app

import (
	"time"

	"simmer/internal/device"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"
	"simmer/internal/viewstate"

	tea "charm.land/bubbletea/v2"
)

// The View state and the Active instance (CONTEXT.md, ADR-0002): this file
// records where the user is, resumes it at launch, and tracks whether this
// Simmer still owns it.

const (
	// viewSaveDebounce is how long the view must sit still before it is
	// written — also how far behind a takeover can resume.
	viewSaveDebounce = 250 * time.Millisecond
	// ownerCheckInterval bounds how long a superseded Simmer goes on
	// believing it is live.
	ownerCheckInterval = time.Second
)

type (
	viewSaveTickMsg  struct{ seq int }
	viewSavedMsg     struct{ owned bool }
	ownerCheckTick   struct{}
	ownerCheckMsg    struct{ owned bool }
	takeOverDoneMsg  struct{ err error }
	startupNoticeMsg struct{ text string }
)

// restoreStage is the next step of resuming the View state. Each step waits
// for the data it needs; any user input abandons the rest.
type restoreStage int

const (
	restoreAwaitDevices restoreStage = iota
	restoreAwaitApps
	restoreAwaitTables
)

type restorePlan struct {
	view  viewstate.View
	stage restoreStage
}

// snapshot reads the View state off the model. It is a pure function of
// what is on screen, so recording is just "save when the snapshot changes".
func (m model) snapshot() viewstate.View {
	v := viewstate.View{Focus: viewstate.FocusSidebar}
	if m.focus == focusMain {
		v.Focus = viewstate.FocusMain
	}
	if d := m.mainPane.ActiveDevice(); d != nil {
		v.DeviceID = d.ID
		v.Loaded = true
		v.Panel = viewstate.PanelApps
		if m.mainPane.IsFilesPanel() {
			v.Panel = viewstate.PanelFiles
		}
		if a := m.mainPane.SelectedApp(); a != nil {
			v.AppCursor = a.BundleID
		}
		if a := m.mainPane.PinnedApp(); a != nil {
			v.PinnedApp = a.BundleID
		}
	} else if d := m.sidebar.SelectedDevice(); d != nil {
		v.DeviceID = d.ID
	}
	if m.dbViewerModal != nil {
		dev, pkg, path, name := m.dbViewerModal.File()
		if path != "" {
			v.DB = &viewstate.DB{
				DeviceID:  dev.ID,
				PackageID: pkg,
				Path:      path,
				Name:      name,
				Query:     m.dbViewerModal.QueryText(),
				Table:     m.dbViewerModal.SelectedTable(),
			}
		}
	}
	return v
}

func sameView(a, b viewstate.View) bool {
	if (a.DB == nil) != (b.DB == nil) {
		return false
	}
	if a.DB != nil && *a.DB != *b.DB {
		return false
	}
	a.DB, b.DB = nil, nil
	return a == b
}

// recording reports whether changes to the view should be written: only by
// the Active instance, and not while a resume is still filling the screen in
// (a half-restored screen would overwrite the state being restored).
func (m model) recording() bool {
	return m.views != nil && m.live && m.restore == nil
}

// isUserInput reports whether msg is the user acting, which abandons a
// resume in progress. Quitting is not: leaving before the resume finishes
// must keep the state, not record a half-built screen.
func isUserInput(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		s := msg.String()
		return s != "q" && s != "ctrl+c" && s != "ctrl+t"
	case tea.MouseClickMsg, tea.MouseWheelMsg:
		return true
	}
	return false
}

// handleInstanceMsg owns the messages and inputs that concern the Active
// instance. It runs before the regular Update so a superseded Simmer can
// take over from inside any overlay.
func (m model) handleInstanceMsg(msg tea.Msg) (model, tea.Cmd, bool) {
	if m.views == nil {
		return m, nil, false
	}
	switch msg := msg.(type) {
	case startupNoticeMsg:
		return m, m.setStatus(msg.text, ui.StatusWarn), true

	case viewSaveTickMsg:
		if msg.seq != m.saveSeq || !m.recording() {
			return m, nil, true
		}
		store, v := m.views, m.recorded
		return m, func() tea.Msg {
			owned, err := store.Save(v)
			if err != nil {
				// A failed write says nothing about ownership.
				return viewSavedMsg{owned: true}
			}
			return viewSavedMsg{owned: owned}
		}, true

	case viewSavedMsg:
		if !msg.owned {
			return m.becomeSuperseded()
		}
		return m, nil, true

	case ownerCheckTick:
		next := tea.Tick(ownerCheckInterval, func(time.Time) tea.Msg { return ownerCheckTick{} })
		if !m.live {
			return m, next, true
		}
		store := m.views
		return m, tea.Batch(next, func() tea.Msg {
			owned, err := store.Owned()
			if err != nil {
				return ownerCheckMsg{owned: true}
			}
			return ownerCheckMsg{owned: owned}
		}), true

	case ownerCheckMsg:
		if !msg.owned && m.live {
			return m.becomeSuperseded()
		}
		return m, nil, true

	case takeOverDoneMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("take over failed: "+errPreview(msg.err), ui.StatusErr), true
		}
		m.live = true
		return m, m.setStatus("this Simmer is live", ui.StatusOk), true

	case tea.KeyPressMsg:
		// Only a superseded Simmer claims ctrl+t: while live it stays the
		// editor's transpose key.
		if msg.String() == "ctrl+t" && !m.live {
			return m.takeOver()
		}

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && !m.live && m.instanceBadgeHit(msg.X, msg.Y) {
			return m.takeOver()
		}
	}
	return m, nil, false
}

func (m model) becomeSuperseded() (model, tea.Cmd, bool) {
	m.live = false
	return m, m.setStatus("another Simmer took over — ^T to take back", ui.StatusWarn), true
}

// takeOver makes this Simmer the Active instance again, keeping what it
// shows now as the View state.
func (m model) takeOver() (model, tea.Cmd, bool) {
	store := m.views
	v := m.snapshot()
	m.recorded = v
	m.restore = nil
	return m, func() tea.Msg { return takeOverDoneMsg{err: store.TakeOver(v)} }, true
}

func (m model) instanceBadgeHit(x, y int) bool {
	if m.dbViewerModal != nil {
		return m.dbViewerModal.LiveBadgeHit(x, y)
	}
	if y != 0 {
		return false
	}
	x0, x1, ok := ui.TopBarInstanceSpan(m.topBarParams())
	return ok && x >= x0 && x < x1
}

func (m model) instanceState() ui.InstanceState {
	switch {
	case m.views == nil:
		return ui.InstanceUnknown
	case m.live:
		return ui.InstanceLive
	default:
		return ui.InstanceSuperseded
	}
}

// scheduleSave arms the debounce when the view changed since the last
// recording.
func (m *model) scheduleSave() tea.Cmd {
	if !m.recording() {
		return nil
	}
	v := m.snapshot()
	if sameView(v, m.recorded) {
		return nil
	}
	m.recorded = v
	m.saveSeq++
	seq := m.saveSeq
	return tea.Tick(viewSaveDebounce, func(time.Time) tea.Msg { return viewSaveTickMsg{seq: seq} })
}

// advanceRestore runs the next resume step once msg has delivered the data
// it waits for. Resuming never boots a device, starts the adb server, or
// runs a query; a part that no longer exists falls back to the nearest part
// that does, with a status saying so.
func (m model) advanceRestore(msg tea.Msg) (model, tea.Cmd) {
	r := m.restore
	if r == nil {
		return m, nil
	}
	v := r.view

	switch msg := msg.(type) {
	case discoveryMsg:
		if r.stage != restoreAwaitDevices {
			return m, nil
		}
		m.restore = nil
		if v.DeviceID == "" || !m.sidebar.SelectDevice(v.DeviceID) {
			if v.DeviceID == "" {
				return m, nil
			}
			return m, m.setStatus("last device not found", ui.StatusWarn)
		}
		dev := m.sidebar.SelectedDevice()
		if !v.Loaded {
			return m, nil
		}
		if dev.Status != device.StatusRunning {
			return m, m.setStatus(dev.Name+" is not running", ui.StatusWarn)
		}
		next, cmd := m.update(ui.LoadDeviceMsg{Device: *dev})
		m = next.(model)
		m.restore = &restorePlan{view: v, stage: restoreAwaitApps}
		return m, cmd

	case appsListMsg:
		active := m.mainPane.ActiveDevice()
		if r.stage != restoreAwaitApps || active == nil || msg.device.ID != active.ID {
			return m, nil
		}
		m.restore = nil
		if v.Focus == viewstate.FocusSidebar {
			m.focus = focusSidebar
			m.applyFocus()
		}
		if msg.err != nil {
			return m, nil
		}
		var cmds []tea.Cmd
		if v.AppCursor != "" && !m.mainPane.SelectAppByID(v.AppCursor) {
			cmds = append(cmds, m.setStatus("last app not found", ui.StatusWarn))
		}
		if v.PinnedApp != "" {
			m.mainPane.PinAppByID(v.PinnedApp)
		}
		if v.Panel == viewstate.PanelFiles {
			if req := m.mainPane.ShowFilesPanel(); req != nil {
				next, cmd := m.update(*req)
				m = next.(model)
				cmds = append(cmds, cmd)
			}
		}
		if db := v.DB; db != nil && db.DeviceID == active.ID {
			next, cmd := m.update(ui.ShowSQLiteViewerMsg{
				Device: *active, PackageID: db.PackageID, DBPath: db.Path, DBName: db.Name,
			})
			m = next.(model)
			m.dbViewerModal.RestoreQuery(db.Query)
			m.dbViewerModal.RestoreTable(db.Table)
			m.restore = &restorePlan{view: v, stage: restoreAwaitTables}
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case dbviewer.TablesLoadedMsg:
		if r.stage != restoreAwaitTables {
			return m, nil
		}
		m.restore = nil
		if msg.Err != nil {
			m.dbViewerModal = nil
			return m, m.setStatus("last database could not be opened", ui.StatusWarn)
		}
	}
	return m, nil
}
