package main

import (
	"context"
	"fmt"
	"os"
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
}

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
		fs:           device.NewMockFileSystem(),
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
		mainW := m.width - (ui.DefaultSidebarWidth + 2) - 1
		if mainW < 0 {
			mainW = 0
		}
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
			if sel.Platform != device.PlatformIOS {
				return m, nil
			}
			return m, m.bootDeviceCmd(*sel)
		case "s":
			sel := m.sidebar.SelectedDevice()
			if sel == nil || sel.Status != device.StatusRunning {
				return m, nil
			}
			if sel.Platform != device.PlatformIOS {
				return m, nil
			}
			return m, m.shutdownDeviceCmd(*sel)
		case "space":
			sel := m.sidebar.SelectedDevice()
			if sel == nil {
				return m, nil
			}
			m.focus = focusMain
			m.applyFocus()
			return m, m.loadFileTreeCmd(*sel)
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
			return m, nil
		}
		m.loading = true
		return m, m.fetchDevicesCmd()

	case shutdownResultMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, nil
		}
		m.loading = true
		return m, m.fetchDevicesCmd()

	case fileTreeMsg:
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, nil
		}
		dev := msg.device
		m.mainPane.SetDevice(&dev, &msg.root)
		return m, nil
	}

	return m, nil
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

	var footerDevice *ui.FooterDevice
	if sel := m.sidebar.SelectedDevice(); sel != nil {
		footerDevice = &ui.FooterDevice{
			Name:     sel.Name,
			Platform: sel.Platform,
			Status:   sel.Status,
		}
	}
	footer := ui.RenderFooter(ui.FooterParams{
		Width:        m.width,
		ActiveDevice: footerDevice,
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
