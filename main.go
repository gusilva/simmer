package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"simmer/pkg/device"
	"simmer/ui"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const appVersion = "v0.0.1"

// ─── Model ────────────────────────────────────────────────────────────────

type model struct {
	list         list.Model
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

func (m model) fetchDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return discoveryMsg(m.coordinator.Discover(ctx))
	}
}

func initialModel() model {
	l := list.New([]list.Item{}, ui.DeviceDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.Styles = ui.NewListStyles()

	coord := device.NewCoordinator(
		device.NewIOSManager(),
		device.NewAndroidManager(),
	)

	return model{
		list:         l,
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
		h, v := ui.StyleDoc.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-4)

	case tea.KeyPressMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			m.loading = true
			m.errs = nil
			return m, m.fetchDevicesCmd()
		}

	case discoveryMsg:
		m.loading = false
		m.errs = msg.Errors
		m.toolVersions = msg.ToolVersions
		m.lastRefresh = time.Now()

		booted, ios, android := 0, 0, 0
		items := make([]list.Item, len(msg.Devices))
		for idx, dev := range msg.Devices {
			items[idx] = ui.Item{Dev: dev}
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
		m.list.SetItems(items)

		var statusCmd tea.Cmd
		if len(m.errs) > 0 {
			statusCmd = m.list.NewStatusMessage(
				lipgloss.NewStyle().Foreground(ui.ColorErr).Render(
					fmt.Sprintf("⚠  %d error(s) during discovery", len(m.errs)),
				),
			)
		} else {
			statusCmd = m.list.NewStatusMessage(
				lipgloss.NewStyle().Foreground(ui.ColorOk).Render("✓  discovery complete"),
			)
		}
		return m, statusCmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	var body string
	if m.loading && len(m.list.Items()) == 0 {
		body = "\n  " + lipgloss.NewStyle().Foreground(ui.ColorFgFaint).Render("fetching devices…")
	} else {
		body = ui.StyleDoc.Render(m.list.View())
	}

	topBar := ui.RenderTopBar(ui.TopBarParams{
		Width:        m.width,
		AppVersion:   appVersion,
		BootedCount:  m.bootedCount,
		IOSCount:     m.iosCount,
		AndroidCount: m.androidCount,
		ToolVersions: m.toolVersions,
	})

	var footerDevice *ui.FooterDevice
	if sel, ok := m.list.SelectedItem().(ui.Item); ok {
		footerDevice = &ui.FooterDevice{
			Name:     sel.Dev.Name,
			Platform: sel.Dev.Platform,
			Status:   sel.Dev.Status,
		}
	}
	footer := ui.RenderFooter(ui.FooterParams{
		Width:        m.width,
		ActiveDevice: footerDevice,
	})

	bodyHeight := max(m.height-lipgloss.Height(topBar)-lipgloss.Height(footer), 0)
	body = lipgloss.NewStyle().Height(bodyHeight).Background(ui.ColorBg).Render(body)

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
