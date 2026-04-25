// Package main implements the Simmer TUI application.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"simmer/pkg/device"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
)

// item implements list.Item for the Bubble Tea list component.
type item struct {
	dev device.Device
}

func (i item) Title() string {
	icon := "📱"
	if i.dev.Platform == device.PlatformAndroid {
		icon = "🤖"
	}
	return fmt.Sprintf("%s %s", icon, i.dev.Name)
}

func (i item) Description() string {
	status := "⚪️ Off"
	if i.dev.Status == device.StatusRunning {
		status = "🟢 Running"
	}
	return fmt.Sprintf("%s | %s | %s", i.dev.Platform, i.dev.Version, status)
}

func (i item) FilterValue() string { return i.dev.Name }

// model represents the application state.
type model struct {
	list        list.Model
	coordinator *device.Coordinator
	loading     bool
	errs        []error
	quitting    bool
}

// discoveryMsg is sent when device discovery completes.
type discoveryMsg device.DiscoveryResult

// fetchDevicesCmd returns a tea.Cmd that performs device discovery.
func (m model) fetchDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return discoveryMsg(m.coordinator.Discover(ctx))
	}
}

func initialModel() model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Simmer"
	l.SetShowStatusBar(true)
	l.Styles.Title = titleStyle

	coord := device.NewCoordinator(
		device.NewIOSManager(),
		device.NewAndroidManager(),
	)

	return model{
		list:        l,
		coordinator: coord,
		loading:     true,
	}
}

func (m model) Init() tea.Cmd {
	return m.fetchDevicesCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

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
		items := make([]list.Item, len(msg.Devices))
		for i, dev := range msg.Devices {
			items[i] = item{dev: dev}
		}
		m.list.SetItems(items)
		
		var statusCmd tea.Cmd
		if len(m.errs) > 0 {
			statusCmd = m.list.NewStatusMessage(fmt.Sprintf("⚠️ %d errors during discovery", len(m.errs)))
		} else {
			statusCmd = m.list.NewStatusMessage("✅ Discovery complete")
		}
		return m, statusCmd

	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	if m.loading && len(m.list.Items()) == 0 {
		return tea.NewView("\n  🔍 Fetching devices...")
	}

	v := tea.NewView(docStyle.Render(m.list.View()))
	v.AltScreen = true
	v.WindowTitle = "Simmer"
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
