package app

import (
	"errors"
	"testing"

	"simmer/internal/device"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"
	"simmer/internal/viewstate"

	tea "charm.land/bubbletea/v2"
)

var (
	vsRunning = device.Device{ID: "dev-1", Name: "Pixel", Platform: device.PlatformAndroid,
		Kind: device.KindVirtual, Status: device.StatusRunning}
	vsOff = device.Device{ID: "dev-2", Name: "iPhone", Platform: device.PlatformIOS,
		Kind: device.KindVirtual, Status: device.StatusOff}
	vsApps = []device.App{{BundleID: "com.a", Name: "A"}, {BundleID: "com.b", Name: "B"}}
)

// newViewStateModel returns a test model that is the Active instance over
// a private state directory, resuming v.
func newViewStateModel(t *testing.T, v viewstate.View) (model, string) {
	t.Helper()
	dir := t.TempDir()
	m := newTestModel()
	m.views = viewstate.Open(dir)
	if _, err := m.views.Claim(false); err != nil {
		t.Fatal(err)
	}
	m.live = true
	m.recorded = v
	if v.DeviceID != "" {
		m.restore = &restorePlan{view: v}
	}
	return m, dir
}

func step(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	res, cmd := m.Update(msg)
	return res.(model), cmd
}

func TestRestore_DeviceAppAndPanel(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{
		DeviceID: vsRunning.ID, Loaded: true, Focus: viewstate.FocusMain,
		Panel: viewstate.PanelApps, AppCursor: "com.b", PinnedApp: "com.a",
	})

	m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning, vsOff}})
	if d := m.mainPane.ActiveDevice(); d == nil || d.ID != vsRunning.ID {
		t.Fatalf("active device = %v, want %s", d, vsRunning.ID)
	}
	if m.restore == nil || m.restore.stage != restoreAwaitApps {
		t.Fatal("restore should wait for the app list")
	}

	m, _ = step(t, m, appsListMsg{device: vsRunning, apps: vsApps})
	if m.restore != nil {
		t.Fatal("restore should be done")
	}
	if a := m.mainPane.SelectedApp(); a == nil || a.BundleID != "com.b" {
		t.Fatalf("app cursor = %v, want com.b", a)
	}
	if a := m.mainPane.PinnedApp(); a == nil || a.BundleID != "com.a" {
		t.Fatalf("pinned = %v, want com.a", a)
	}
	if m.focus != focusMain {
		t.Fatal("focus should be main")
	}
}

func TestRestore_FocusSidebarAfterLoad(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsRunning.ID, Loaded: true, Focus: viewstate.FocusSidebar})
	m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
	m, _ = step(t, m, appsListMsg{device: vsRunning, apps: vsApps})
	if m.focus != focusSidebar {
		t.Fatal("focus should return to the sidebar")
	}
}

func TestRestore_FilesPanelRequestsTree(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsRunning.ID, Loaded: true, Panel: viewstate.PanelFiles})
	m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
	m, cmd := step(t, m, appsListMsg{device: vsRunning, apps: vsApps})
	if !m.mainPane.IsFilesPanel() {
		t.Fatal("files panel should be expanded")
	}
	if cmd == nil {
		t.Fatal("expected a tree load")
	}
}

func TestRestore_NeverBootsAnOffDevice(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsOff.ID, Loaded: true})
	m, cmd := step(t, m, discoveryMsg{Devices: []device.Device{vsRunning, vsOff}})
	if m.booting || m.mainPane.ActiveDevice() != nil {
		t.Fatal("an off device must not be booted or loaded")
	}
	if sel := m.sidebar.SelectedDevice(); sel == nil || sel.ID != vsOff.ID {
		t.Fatalf("sidebar cursor = %v, want %s", sel, vsOff.ID)
	}
	if cmd == nil || m.status != "iPhone is not running" {
		t.Fatalf("status = %q", m.status)
	}
	if m.restore != nil {
		t.Fatal("restore should stop at the device")
	}
}

func TestRestore_MissingPartsFallBack(t *testing.T) {
	t.Run("device", func(t *testing.T) {
		m, _ := newViewStateModel(t, viewstate.View{DeviceID: "gone", Loaded: true})
		m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
		if m.status != "last device not found" || m.restore != nil {
			t.Fatalf("status = %q restore = %v", m.status, m.restore)
		}
	})
	t.Run("app", func(t *testing.T) {
		m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsRunning.ID, Loaded: true, AppCursor: "com.gone"})
		m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
		m, _ = step(t, m, appsListMsg{device: vsRunning, apps: vsApps})
		if m.status != "last app not found" {
			t.Fatalf("status = %q", m.status)
		}
		if a := m.mainPane.SelectedApp(); a == nil || a.BundleID != "com.a" {
			t.Fatal("cursor should stay on the first app")
		}
	})
	t.Run("database", func(t *testing.T) {
		m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsRunning.ID, Loaded: true,
			DB: &viewstate.DB{DeviceID: vsRunning.ID, PackageID: "com.a", Path: "/d/x.db", Name: "x.db", Query: "select 1;"}})
		m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
		m, _ = step(t, m, appsListMsg{device: vsRunning, apps: vsApps})
		if m.dbViewerModal == nil {
			t.Fatal("DB viewer should open")
		}
		if got := m.dbViewerModal.QueryText(); got != "select 1;" {
			t.Fatalf("query = %q", got)
		}
		m, _ = step(t, m, dbviewer.TablesLoadedMsg{Err: errors.New("no such file")})
		if m.dbViewerModal != nil || m.status != "last database could not be opened" {
			t.Fatalf("viewer=%v status=%q", m.dbViewerModal != nil, m.status)
		}
	})
}

func TestRestore_UserInputAbandonsIt(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsRunning.ID, Loaded: true})
	m, _ = step(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.restore != nil {
		t.Fatal("a key press should abandon the restore")
	}
	m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
	if m.mainPane.ActiveDevice() != nil {
		t.Fatal("an abandoned restore must not load the device")
	}
}

func TestRestore_QuitKeepsItPending(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{DeviceID: vsRunning.ID, Loaded: true})
	m, _ = step(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.restore == nil || m.recording() {
		t.Fatal("quitting mid-restore must not record a half-built view")
	}
}

func TestRecording_DebouncedSave(t *testing.T) {
	m, dir := newViewStateModel(t, viewstate.View{})
	m, _ = step(t, m, discoveryMsg{Devices: []device.Device{vsRunning}})
	seq := m.saveSeq
	m, cmd := step(t, m, ui.LoadDeviceMsg{Device: vsRunning})
	if cmd == nil || m.saveSeq != seq+1 {
		t.Fatalf("a view change should arm the debounce (seq=%d)", m.saveSeq)
	}

	// A stale tick does nothing; the current one writes.
	if _, c := step(t, m, viewSaveTickMsg{seq: 0}); c != nil {
		t.Fatal("stale tick should not save")
	}
	_, c := step(t, m, viewSaveTickMsg{seq: m.saveSeq})
	if msg, ok := c().(viewSavedMsg); !ok || !msg.owned {
		t.Fatalf("save result %v", msg)
	}

	got, _ := viewstate.Open(dir).Claim(false)
	if got.DeviceID != vsRunning.ID {
		t.Fatalf("recorded %+v", got)
	}
}

func TestSuperseded_StopsRecordingAndTakesBack(t *testing.T) {
	m, dir := newViewStateModel(t, viewstate.View{})
	other := viewstate.Open(dir)
	if _, err := other.Claim(false); err != nil {
		t.Fatal(err)
	}

	m, _ = step(t, m, ownerCheckMsg{owned: false})
	if m.live || m.instanceState() != ui.InstanceSuperseded {
		t.Fatal("should be superseded")
	}
	m, _ = step(t, m, ui.LoadDeviceMsg{Device: vsRunning})
	if m.saveSeq != 0 {
		t.Fatal("a superseded instance must not record")
	}

	m, cmd := step(t, m, tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+t should take over")
	}
	m, _ = step(t, m, cmd())
	if !m.live {
		t.Fatal("should be live again")
	}
	if owned, _ := other.Owned(); owned {
		t.Fatal("the other instance should now be superseded")
	}
	got, _ := viewstate.Open(dir).Claim(false)
	if got.DeviceID != vsRunning.ID {
		t.Fatalf("take-over should record what this instance shows, got %+v", got)
	}
}

func TestLive_CtrlTStaysWithTheEditor(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{})
	if _, _, handled := m.handleInstanceMsg(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl}); handled {
		t.Fatal("a live instance must not claim ctrl+t")
	}
}

func TestSuperseded_TopBarClickTakesOver(t *testing.T) {
	m, _ := newViewStateModel(t, viewstate.View{})
	m.width, m.height = 120, 40
	m.live = false
	x0, _, ok := ui.TopBarInstanceSpan(m.topBarParams())
	if !ok {
		t.Fatal("indicator should be drawn")
	}
	if _, cmd := step(t, m, tea.MouseClickMsg{Button: tea.MouseLeft, X: x0, Y: 0}); cmd == nil {
		t.Fatal("clicking the indicator should take over")
	}
}
