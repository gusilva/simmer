package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"simmer/internal/device"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ─── Model ────────────────────────────────────────────────────────────────

type appFocus int

const (
	focusSidebar appFocus = iota
	focusMain
)

type model struct {
	sidebar      ui.Sidebar
	mainPane     ui.MainPane
	focus        appFocus
	coordinator  *device.Coordinator
	loading      bool
	errs         []error
	quitting     bool
	width        int
	height       int
	toolVersions map[device.Platform]string
	bootedCount  int
	iosCount     int
	androidCount int
	lastRefresh  time.Time
	appVersion   string

	logStream   *device.LogStream
	logBundleID string
	logDeviceID string

	status     string
	statusKind ui.StatusKind
	statusSeq  int

	platformPicker *ui.PlatformPickerModal
	createIOSModal *ui.CreateSimulatorModal
	createAndModal *ui.CreateAndroidEmulatorModal
	deleteAlert    *ui.DeleteSimulatorAlert
	sqliteModal    *ui.SQLiteModal
	dbViewerModal  *dbviewer.Modal
}

type clearStatusMsg int

type autoRefreshMsg struct{}

type discoveryMsg device.DiscoveryResult

type deviceTypesMsg struct {
	types []device.DeviceType
	err   error
}

type runtimesMsg struct {
	runtimes []device.Runtime
	err      error
}

type createSimulatorResultMsg struct {
	name string
	udid string
	err  error
}

type deleteSimulatorResultMsg struct {
	name string
	err  error
}

type androidSystemImagesMsg struct {
	images []device.Runtime
	err    error
}

type androidDeviceProfilesMsg struct {
	profiles []device.DeviceType
	err      error
}

type createAndroidEmulatorResultMsg struct {
	name string
	err  error
}

type bootResultMsg struct {
	device device.Device
	err    error
}

type shutdownResultMsg struct {
	device device.Device
	err    error
}

type fileTreeMsg struct {
	device device.Device
	root   device.FileNode
	err    error
}

type appsListMsg struct {
	device device.Device
	apps   []device.App
	err    error
}

type infoMsg struct {
	device device.Device
	info   device.DeviceInfo
	err    error
}

type bootPollMsg struct {
	device    device.Device
	remaining int
}

type logLineMsg struct {
	bundleID string
	line     string
}

type logEndedMsg struct {
	bundleID string
	err      error
}

func (m model) fetchDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return discoveryMsg(m.coordinator.Discover(ctx))
	}
}

func (m model) fetchDeviceTypesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		types, err := m.coordinator.ListDeviceTypes(ctx, device.PlatformIOS)
		return deviceTypesMsg{types: types, err: err}
	}
}

func (m model) fetchRuntimesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		runtimes, err := m.coordinator.ListRuntimes(ctx, device.PlatformIOS)
		return runtimesMsg{runtimes: runtimes, err: err}
	}
}

func (m model) createIOSSimulatorCmd(name, deviceTypeID, runtimeID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		udid, err := m.coordinator.Create(ctx, device.PlatformIOS, name, deviceTypeID, runtimeID)
		return createSimulatorResultMsg{name: name, udid: udid, err: err}
	}
}

func (m model) deleteDeviceCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := m.coordinator.Delete(ctx, dev)
		return deleteSimulatorResultMsg{name: dev.Name, err: err}
	}
}

func (m model) fetchAndroidSystemImagesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		imgs, err := m.coordinator.ListRuntimes(ctx, device.PlatformAndroid)
		return androidSystemImagesMsg{images: imgs, err: err}
	}
}

func (m model) fetchAndroidDeviceProfilesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		profiles, err := m.coordinator.ListDeviceTypes(ctx, device.PlatformAndroid)
		return androidDeviceProfilesMsg{profiles: profiles, err: err}
	}
}

func (m model) createAndroidEmulatorCmd(name, systemImagePkg, deviceProfileID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_, err := m.coordinator.Create(ctx, device.PlatformAndroid, name, deviceProfileID, systemImagePkg)
		return createAndroidEmulatorResultMsg{name: name, err: err}
	}
}

func (m model) bootDeviceCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return bootResultMsg{device: dev, err: m.coordinator.Boot(ctx, dev)}
	}
}

func (m model) shutdownDeviceCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return shutdownResultMsg{device: dev, err: m.coordinator.Shutdown(ctx, dev)}
	}
}

func (m model) loadIOSAppFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fs := device.NewIOSAppFileSystem(app.BundleID)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadIOSRootFileTreeCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fs := device.NewIOSRootFileSystem()
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		fs := device.NewAndroidFileSystem(app.BundleID)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidRootFileTreeCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fs := device.NewAndroidRootFileSystem()
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAppsCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		apps, err := m.coordinator.ListApps(ctx, dev)

		return appsListMsg{device: dev, apps: apps, err: err}
	}
}

func (m model) loadInfoCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		info, err := m.coordinator.Info(ctx, dev)
		return infoMsg{device: dev, info: info, err: err}
	}
}

// nextLogLineCmd reads one line from the active log stream and returns the
// matching tea.Msg. The bundleID is carried in the msg so the handler can
// drop stale messages from a stream that has since been replaced.
// scheduleBootPoll returns a Cmd that fires bootPollMsg after 4 seconds.
// Used to keep refreshing after an Android emulator boot until adb sees it.
func scheduleBootPoll(dev device.Device, remaining int) tea.Cmd {
	if remaining <= 0 {
		return nil
	}
	return tea.Tick(4*time.Second, func(_ time.Time) tea.Msg {
		return bootPollMsg{device: dev, remaining: remaining}
	})
}

func nextLogLineCmd(stream *device.LogStream, bundleID string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-stream.Lines
		if !ok {
			err := <-stream.Done
			return logEndedMsg{bundleID: bundleID, err: err}
		}
		return logLineMsg{bundleID: bundleID, line: line}
	}
}

func initialModel(version string) model {
	coord := device.NewCoordinator(
		device.NewIOSManager(),
		device.NewAndroidManager(),
	)

	m := model{
		sidebar:      ui.NewSidebar(),
		mainPane:     ui.NewMainPane(),
		focus:        focusSidebar,
		coordinator:  coord,
		loading:      true,
		toolVersions: make(map[device.Platform]string),
		appVersion:   version,
	}
	m.applyFocus()
	return m
}

// applyFocus syncs the focus flag onto the sub-components.
func (m *model) applyFocus() {
	m.sidebar.SetFocused(m.focus == focusSidebar)
	m.mainPane.SetFocused(m.focus == focusMain)
}

// setStatus stamps a transient message onto the footer's right side and
// returns a tea.Cmd that clears it after a few seconds. Each call bumps a
// sequence counter so the auto-clear only fires for the last status.
func (m *model) setStatus(text string, kind ui.StatusKind) tea.Cmd {
	m.statusSeq++
	m.status = text
	m.statusKind = kind
	seq := m.statusSeq

	return tea.Tick(4*time.Second, func(_ time.Time) tea.Msg {
		return clearStatusMsg(seq)
	})
}

const autoRefreshInterval = 30 * time.Second

func scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(_ time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchDevicesCmd(), scheduleAutoRefresh())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Body wrapper applies 1 line of top padding around each panel; pass
		// the remaining height so panels fill exactly.
		bodyH := m.bodyHeight() - 1
		m.sidebar.SetSize(ui.DefaultSidebarWidth, bodyH)
		// Sidebar wrapper: pad 1 left + 1 right around DefaultSidebarWidth.
		// MainPane wrapper: pad 0 left + 1 right around its width.
		mainW := max(m.width-(ui.DefaultSidebarWidth+2)-1, 0)
		m.mainPane.SetSize(mainW, bodyH)
		if m.sqliteModal != nil {
			m.sqliteModal.SetSize(m.width, m.height)
		}
		if m.dbViewerModal != nil {
			m.dbViewerModal.SetSize(m.width, m.height)
		}
		return m, nil

	case tea.KeyPressMsg:
		// ctrl+c always quits, even when an overlay is active.
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// ctrl+d toggles the DB viewer regardless of focus or overlay state.
		if msg.String() == "ctrl+d" {
			if m.dbViewerModal != nil {
				m.dbViewerModal = nil
				return m, nil
			}
			if m.platformPicker == nil && m.createIOSModal == nil && m.createAndModal == nil && m.deleteAlert == nil && m.sqliteModal == nil {
				modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
				modal.SetSize(m.width, m.height)
				m.dbViewerModal = &modal
			}
			return m, nil
		}

		// Overlay intercepts all other keys when active.
		if m.platformPicker != nil {
			updated, cmd := m.platformPicker.Update(msg)
			m.platformPicker = &updated
			return m, cmd
		}
		if m.createIOSModal != nil {
			updated, cmd := m.createIOSModal.Update(msg)
			m.createIOSModal = &updated
			return m, cmd
		}
		if m.createAndModal != nil {
			updated, cmd := m.createAndModal.Update(msg)
			m.createAndModal = &updated
			return m, cmd
		}
		if m.deleteAlert != nil {
			updated, cmd := m.deleteAlert.Update(msg)
			m.deleteAlert = &updated
			return m, cmd
		}
		if m.sqliteModal != nil {
			updated, cmd := m.sqliteModal.Update(msg)
			m.sqliteModal = &updated
			return m, cmd
		}
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated
			return m, cmd
		}

		// Global keys (no overlay active).
		if msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

		if m.focus == focusMain {
			if msg.String() == "esc" {
				m.focus = focusSidebar
				m.applyFocus()
				return m, nil
			}
			var cmd tea.Cmd
			m.mainPane, cmd = m.mainPane.Update(msg)
			return m, cmd
		}

		// focus == focusSidebar
		switch msg.String() {
		case "r":
			m.loading = true
			m.errs = nil
			return m, m.fetchDevicesCmd()
		case "b":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status == device.StatusRunning {
				return m, nil
			}
			return m, m.bootDeviceCmd(*sel)
		case "s":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status != device.StatusRunning {
				return m, nil
			}
			return m, m.shutdownDeviceCmd(*sel)
		case "space":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status != device.StatusRunning {
				return m, nil
			}

			m.focus = focusMain
			m.applyFocus()

			dev := *sel
			m.mainPane.SetDevice(&dev, nil)

			return m, tea.Batch(m.loadInfoCmd(dev), m.loadAppsCmd(dev))
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)

		return m, cmd

	case tea.PasteMsg:
		if m.sqliteModal != nil {
			updated, cmd := m.sqliteModal.Update(msg)
			m.sqliteModal = &updated
			return m, cmd
		}

		return m, nil

	case autoRefreshMsg:
		return m, tea.Batch(m.fetchDevicesCmd(), scheduleAutoRefresh())

	case discoveryMsg:
		m.loading = false
		m.errs = msg.Errors
		m.toolVersions = msg.ToolVersions
		m.lastRefresh = time.Now()

		booted, ios, android := 0, 0, 0
		for _, dev := range msg.Devices {
			if dev.Status == device.StatusRunning {
				booted++
			}
			switch dev.Platform {
			case device.PlatformIOS:
				ios++
			case device.PlatformAndroid:
				android++
			}
		}
		m.bootedCount = booted
		m.iosCount = ios
		m.androidCount = android
		m.sidebar.SetDevices(msg.Devices)
		m.mainPane.SyncActiveDevice(msg.Devices)
		return m, nil

	case bootResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("boot failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.loading = true

		cmds := []tea.Cmd{
			m.fetchDevicesCmd(),
			m.setStatus("booted "+msg.device.Name, ui.StatusOk),
		}

		if msg.device.Platform == device.PlatformAndroid {
			// Emulator registers with adb asynchronously; poll until it appears.
			cmds = append(cmds, scheduleBootPoll(msg.device, 15))
		}

		return m, tea.Batch(cmds...)

	case bootPollMsg:
		// Check if the device now shows as running; if so, stop polling.
		for _, dev := range m.sidebar.Devices() {
			if dev.ID == msg.device.ID && dev.Status == device.StatusRunning {
				return m, nil
			}
		}

		return m, tea.Batch(
			m.fetchDevicesCmd(),
			scheduleBootPoll(msg.device, msg.remaining-1),
		)

	case shutdownResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("shutdown failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.loading = true

		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("shut down "+msg.device.Name, ui.StatusOk),
		)

	case fileTreeMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("files load failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.mainPane.SetTree(&msg.root)

		return m, nil

	case appsListMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("apps load failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.mainPane.SetApps(msg.apps)

		return m, nil

	case infoMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)

			return m, m.setStatus("info load failed: "+errPreview(msg.err), ui.StatusErr)
		}

		m.mainPane.SetInfo(msg.info)

		return m, nil

	case ui.ClipboardCopiedMsg:
		if msg.Err != nil {
			m.errs = append(m.errs, msg.Err)

			return m, m.setStatus("copy failed: "+errPreview(msg.Err), ui.StatusErr)
		}

		return m, m.setStatus("copied "+textPreview(msg.Text, 40), ui.StatusOk)

	case ui.AppFocusedMsg:
		return m, m.setStatus("app: "+msg.App.Label(), ui.StatusInfo)

	case clearStatusMsg:
		if int(msg) == m.statusSeq {
			m.status = ""
		}

		return m, nil

	case ui.RequestFileTreeMsg:
		sel := m.sidebar.SelectedDevice()
		if sel == nil {
			return m, nil
		}

		if msg.App == nil {
			switch sel.Platform {
			case device.PlatformIOS:
				return m, m.loadIOSRootFileTreeCmd(*sel)
			case device.PlatformAndroid:
				return m, m.loadAndroidRootFileTreeCmd(*sel)
			}
			return m, nil
		}

		switch sel.Platform {
		case device.PlatformIOS:
			return m, m.loadIOSAppFileTreeCmd(*sel, *msg.App)
		case device.PlatformAndroid:
			return m, m.loadAndroidFileTreeCmd(*sel, *msg.App)
		}

		return m, nil

	case ui.StopLogStreamMsg:
		if m.logStream != nil {
			m.logStream.Stop()
			m.logStream = nil
		}
		m.logBundleID = ""
		m.logDeviceID = ""
		m.mainPane.SetLogBundle("")

		return m, nil

	case ui.RequestLogStreamMsg:
		sel := m.sidebar.SelectedDevice()
		if sel == nil {
			return m, nil
		}
		bundleID := msg.App.BundleID
		if m.logStream != nil && m.logBundleID == bundleID && m.logDeviceID == sel.ID {
			return m, nil
		}
		if m.logStream != nil {
			m.logStream.Stop()
			m.logStream = nil
		}
		stream, err := m.coordinator.StreamLogs(context.Background(), *sel, msg.App)
		if err != nil {
			m.errs = append(m.errs, err)
			return m, m.setStatus("stream failed: "+errPreview(err), ui.StatusErr)
		}
		m.logStream = stream
		m.logBundleID = bundleID
		m.logDeviceID = sel.ID
		m.mainPane.SetLogBundle(bundleID)
		return m, tea.Batch(
			nextLogLineCmd(stream, bundleID),
			m.setStatus("streaming "+bundleID, ui.StatusOk),
		)

	case logLineMsg:
		if msg.bundleID != m.logBundleID || m.logStream == nil {
			return m, nil
		}
		m.mainPane.AppendLog(msg.line)
		return m, nextLogLineCmd(m.logStream, m.logBundleID)

	case logEndedMsg:
		if msg.bundleID != m.logBundleID {
			return m, nil
		}
		m.logStream = nil
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("stream ended: "+errPreview(msg.err), ui.StatusWarn)
		}
		return m, m.setStatus("stream ended", ui.StatusInfo)

	case ui.ShowPlatformPickerMsg:
		p := ui.NewPlatformPickerModal()
		m.platformPicker = &p
		return m, nil

	case ui.ConfirmPlatformPickerMsg:
		m.platformPicker = nil
		if msg.Platform == device.PlatformIOS {
			modal, focusCmd := ui.NewCreateSimulatorModal()
			m.createIOSModal = &modal
			return m, tea.Batch(focusCmd, m.fetchDeviceTypesCmd(), m.fetchRuntimesCmd())
		}
		andModal, focusCmd := ui.NewCreateAndroidEmulatorModal()
		m.createAndModal = &andModal
		return m, tea.Batch(focusCmd, m.fetchAndroidSystemImagesCmd(), m.fetchAndroidDeviceProfilesCmd())

	case ui.ShowDeleteSimulatorMsg:
		alert := ui.NewDeleteSimulatorAlert(msg.Device)
		m.deleteAlert = &alert
		return m, nil

	case ui.ShowSQLiteViewerMsg:
		modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
		modal.SetSize(m.width, m.height)

		fileCmd := modal.SetFile(msg.Device, msg.PackageID, msg.DBPath, msg.DBName)

		m.dbViewerModal = &modal
		m.sqliteModal = nil

		return m, fileCmd

	case ui.SQLiteResultMsg:
		if m.sqliteModal != nil {
			m.sqliteModal.SetResult(msg.Rows, msg.Err)
		}
		return m, nil

	case dbviewer.SQLiteVersionMsg, dbviewer.TablesLoadedMsg, dbviewer.ColumnsLoadedMsg:
		if m.dbViewerModal != nil {
			updated, cmd := m.dbViewerModal.Update(msg)
			m.dbViewerModal = &updated

			return m, cmd
		}

		return m, nil

	case dbviewer.ShowMsg:
		modal := dbviewer.New(func() tea.Msg { return ui.CancelOverlayMsg{} })
		modal.SetSize(m.width, m.height)
		m.dbViewerModal = &modal
		return m, nil

	case ui.CancelOverlayMsg:
		m.platformPicker = nil
		m.createIOSModal = nil
		m.createAndModal = nil
		m.deleteAlert = nil
		m.sqliteModal = nil
		m.dbViewerModal = nil
		return m, nil

	case ui.ConfirmCreateSimulatorMsg:
		m.createIOSModal = nil
		return m, m.createIOSSimulatorCmd(msg.Name, msg.DeviceTypeID, msg.RuntimeID)

	case ui.ConfirmCreateAndroidEmulatorMsg:
		m.createAndModal = nil
		return m, m.createAndroidEmulatorCmd(msg.Name, msg.SystemImagePkg, msg.DeviceProfileID)

	case ui.ConfirmDeleteSimulatorMsg:
		m.deleteAlert = nil
		return m, m.deleteDeviceCmd(msg.Device)

	case deviceTypesMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			if m.createIOSModal != nil {
				m.createIOSModal.SetDeviceTypes(nil)
			}
			return m, m.setStatus("device types: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.createIOSModal != nil {
			m.createIOSModal.SetDeviceTypes(msg.types)
		}
		return m, nil

	case runtimesMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			if m.createIOSModal != nil {
				m.createIOSModal.SetRuntimes(nil)
			}
			return m, m.setStatus("runtimes: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.createIOSModal != nil {
			m.createIOSModal.SetRuntimes(msg.runtimes)
		}
		return m, nil

	case androidSystemImagesMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			if m.createAndModal != nil {
				m.createAndModal.SetSystemImages(nil)
			}
			return m, m.setStatus("system images: "+errPreview(msg.err), ui.StatusErr)
		}
		if m.createAndModal != nil {
			m.createAndModal.SetSystemImages(msg.images)
		}
		return m, nil

	case androidDeviceProfilesMsg:
		if msg.err != nil {
			// Device profiles are optional; log but don't block
			if m.createAndModal != nil {
				m.createAndModal.SetDeviceProfiles(nil)
			}
			return m, nil
		}
		if m.createAndModal != nil {
			m.createAndModal.SetDeviceProfiles(msg.profiles)
		}
		return m, nil

	case createSimulatorResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("create failed: "+errPreview(msg.err), ui.StatusErr)
		}
		m.loading = true
		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("created "+msg.name, ui.StatusOk),
		)

	case createAndroidEmulatorResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("create failed: "+errPreview(msg.err), ui.StatusErr)
		}
		m.loading = true
		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("created "+msg.name, ui.StatusOk),
		)

	case deleteSimulatorResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, m.setStatus("delete failed: "+errPreview(msg.err), ui.StatusErr)
		}
		m.loading = true
		return m, tea.Batch(
			m.fetchDevicesCmd(),
			m.setStatus("deleted "+msg.name, ui.StatusOk),
		)
	}

	return m, nil
}

// errPreview returns a short, single-line excerpt of err's message, suitable
// for the status bar.
func errPreview(err error) string {
	if err == nil {
		return ""
	}

	return textPreview(err.Error(), 60)
}

// textPreview clips s to max visible characters with a trailing ellipsis.
func textPreview(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

func (m model) bodyHeight() int {
	topBar := ui.RenderTopBar(ui.TopBarParams{Width: m.width})
	footer := ui.RenderFooter(ui.FooterParams{Width: m.width})
	return max(m.height-lipgloss.Height(topBar)-lipgloss.Height(footer), 0)
}

func (m model) View() tea.View {
	topBar := ui.RenderTopBar(ui.TopBarParams{
		Width:        m.width,
		AppVersion:   m.appVersion,
		BootedCount:  m.bootedCount,
		IOSCount:     m.iosCount,
		AndroidCount: m.androidCount,
		ToolVersions: m.toolVersions,
	})

	footer := ui.RenderFooter(ui.FooterParams{
		Width:  m.width,
		Status: m.status,
		Kind:   m.statusKind,
	})

	bodyH := max(m.height-lipgloss.Height(topBar)-lipgloss.Height(footer), 0)

	var body string
	if m.loading && m.sidebar.SelectedDevice() == nil {
		body = "\n  " + lipgloss.NewStyle().
			Foreground(ui.ColorFgFaint).
			Background(ui.ColorBg).
			Render("fetching devices…")
	} else {
		sidebar := lipgloss.NewStyle().
			Padding(1, 1, 0, 1).
			Background(ui.ColorBg).
			Render(m.sidebar.View())
		mainPane := lipgloss.NewStyle().
			Padding(1, 1, 0, 0).
			Background(ui.ColorBg).
			Render(m.mainPane.View())
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPane)
	}

	body = lipgloss.NewStyle().
		Width(m.width).
		Height(bodyH).
		Background(ui.ColorBg).
		Render(body)

	baseStr := lipgloss.JoinVertical(lipgloss.Left, topBar, body, footer)

	if m.platformPicker != nil || m.createIOSModal != nil || m.createAndModal != nil || m.deleteAlert != nil || m.sqliteModal != nil || m.dbViewerModal != nil {
		var overlayStr string
		switch {
		case m.dbViewerModal != nil:
			overlayStr = m.dbViewerModal.View()
		case m.platformPicker != nil:
			overlayStr = m.platformPicker.View()
		case m.createIOSModal != nil:
			overlayStr = m.createIOSModal.View()
		case m.createAndModal != nil:
			overlayStr = m.createAndModal.View()
		case m.sqliteModal != nil:
			overlayStr = m.sqliteModal.View()
		default:
			overlayStr = m.deleteAlert.View()
		}
		mW := lipgloss.Width(overlayStr)
		mH := lipgloss.Height(overlayStr)
		x := max((m.width-mW)/2, 0)
		y := max((m.height-mH)/2, 0)
		bg := lipgloss.NewLayer(baseStr)
		fg := lipgloss.NewLayer(overlayStr).X(x).Y(y).Z(1)
		if m.dbViewerModal != nil {
			scrim := lipgloss.NewLayer(m.dbViewerModal.ScrimView()).Z(0)
			baseStr = lipgloss.NewCompositor(bg, scrim, fg).Render()
		} else {
			baseStr = lipgloss.NewCompositor(bg, fg).Render()
		}
	}

	v := tea.NewView(baseStr)
	v.AltScreen = true
	v.WindowTitle = "Simmer"
	v.BackgroundColor = ui.ColorBg
	return v
}

func Run(version string) error {
	m := initialModel(version)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run program: %w", err)
	}

	return nil
}
