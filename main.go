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

type model struct {
	sidebar      ui.Sidebar
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

func initialModel() model {
	coord := device.NewCoordinator(
		device.NewIOSManager(),
		device.NewAndroidManager(),
	)

	return model{
		sidebar:      ui.NewSidebar(),
		coordinator:  coord,
		loading:      true,
		toolVersions: make(map[device.Platform]string),
	}
}

func (m model) Init() tea.Cmd {
	return m.fetchDevicesCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Body wrapper applies 1 line of top padding around the sidebar; pass
		// the remaining height so the Available panel can fill exactly.
		m.sidebar.SetSize(ui.DefaultSidebarWidth, m.bodyHeight()-1)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
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
		rest := lipgloss.NewStyle().
			Background(ui.ColorBg).
			Width(max(m.width-lipgloss.Width(sidebar), 0)).
			Render("")
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, rest)
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
