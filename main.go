package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"simmer/pkg/device"
	"simmer/ui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const appVersion = "v0.0.1"

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
	fs           device.FileSystem
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

	logStream   *device.LogStream
	logBundleID string
	logDeviceID string

	status     string
	statusKind ui.StatusKind
	statusSeq  int
}

type clearStatusMsg int

type discoveryMsg device.DiscoveryResult

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

func (m model) loadFileTreeCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		root, err := m.fs.Tree(ctx, dev)
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

func initialModel() model {
	coord := device.NewCoordinator(
		device.NewIOSManager(),
		device.NewAndroidManager(),
	)

	m := model{
		sidebar:      ui.NewSidebar(),
		mainPane:     ui.NewMainPane(),
		focus:        focusSidebar,
		coordinator:  coord,
		fs:           device.NewIOSFileSystem(),
		loading:      true,
		toolVersions: make(map[device.Platform]string),
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

func (m model) Init() tea.Cmd {
	return m.fetchDevicesCmd()
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
		return m, nil

	case tea.KeyPressMsg:
		// Global keys — fire regardless of focus.
		switch msg.String() {
		case "q", "ctrl+c":
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

			return m, tea.Batch(
				m.loadFileTreeCmd(*sel),
				m.loadAppsCmd(*sel),
				m.loadInfoCmd(*sel),
			)
		}
		var cmd tea.Cmd
		m.sidebar, cmd = m.sidebar.Update(msg)
		return m, cmd

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

		dev := msg.device
		m.mainPane.SetDevice(&dev, &msg.root)

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
		AppVersion:   appVersion,
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

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, topBar, body, footer))
	v.AltScreen = true
	v.WindowTitle = "Simmer"
	v.BackgroundColor = ui.ColorBg
	return v
}

func main() {
	m := initialModel()
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}
}
