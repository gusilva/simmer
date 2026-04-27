package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"simmer/pkg/device"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const appVersion = "v0.4.2"

// ─── Design System: Color Tokens ──────────────────────────────────────────

var (
	// Surfaces
	colorBg = lipgloss.Color("#0d0e16")

	// Borders
	colorBorder   = lipgloss.Color("#2a2c3a")
	colorBorderHi = lipgloss.Color("#cba6f7")

	// Foreground
	colorFg      = lipgloss.Color("#e6e6f0")
	colorFgDim   = lipgloss.Color("#9da0b3")
	colorFgFaint = lipgloss.Color("#5a5d72")

	// Accents
	colorAccent  = lipgloss.Color("#cba6f7")
	colorAccent2 = lipgloss.Color("#89dceb")

	// Semantic
	colorOk  = lipgloss.Color("#a6e3a1")
	colorErr = lipgloss.Color("#f38ba8")

	// Platform
	colorIOS     = lipgloss.Color("#89dceb")
	colorAndroid = lipgloss.Color("#a6e3a1")
)

// ─── Shared Styles ────────────────────────────────────────────────────────

var (
	styleDoc = lipgloss.NewStyle().Margin(1, 2).Background(colorBg)

	styleTopBar = lipgloss.NewStyle().
			Background(colorBg).
			Padding(0, 1)

	styleBrand = lipgloss.NewStyle().
			Foreground(colorAccent).
			Background(colorBg).
			Bold(true)

	styleFaint = lipgloss.NewStyle().
			Foreground(colorFgFaint).
			Background(colorBg)

	styleDim = lipgloss.NewStyle().
			Foreground(colorFgDim).
			Background(colorBg)
)

// ─── List Item ────────────────────────────────────────────────────────────

type item struct{ dev device.Device }

func (i item) FilterValue() string { return i.dev.Name }
func (i item) Title() string       { return i.dev.Name }
func (i item) Description() string { return string(i.dev.Platform) + " " + i.dev.Version }

// ─── Device Delegate ──────────────────────────────────────────────────────

type deviceDelegate struct{}

func (d deviceDelegate) Height() int                             { return 1 }
func (d deviceDelegate) Spacing() int                            { return 0 }
func (d deviceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d deviceDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}
	dev := i.dev
	isSelected := index == m.Index()
	width := m.Width()

	dotC := colorFgFaint
	if dev.Status == device.StatusRunning {
		dotC = colorOk
	}

	pglyph := ""
	pgC := colorIOS
	if dev.Platform != device.PlatformIOS {
		pglyph = "▲"
		pgC = colorAndroid
	}

	ver := dev.Version

	if isSelected {
		left := "● " + pglyph + " " + dev.Name
		avail := width - 2 // subtract padding (1 char each side)
		gap := avail - lipgloss.Width(left) - lipgloss.Width(ver)
		if gap < 1 {
			gap = 1
		}
		fmt.Fprint(w, lipgloss.NewStyle().
			Background(colorAccent).
			Foreground(colorBg).
			Bold(true).
			Padding(0, 1).
			Render(left+strings.Repeat(" ", gap)+ver))
	} else {
		dot := lipgloss.NewStyle().Foreground(dotC).Render("●")
		glyph := lipgloss.NewStyle().Foreground(pgC).Render(pglyph)
		name := lipgloss.NewStyle().Foreground(colorFg).Render(dev.Name)
		meta := lipgloss.NewStyle().Foreground(colorFgFaint).Render(ver)
		left := dot + " " + glyph + " " + name
		avail := width - 2
		gap := avail - lipgloss.Width(left) - lipgloss.Width(ver)
		if gap < 1 {
			gap = 1
		}
		fmt.Fprint(w, lipgloss.NewStyle().
			Background(colorBg).
			Padding(0, 1).
			Render(left+strings.Repeat(" ", gap)+meta))
	}
}

// ─── Model ────────────────────────────────────────────────────────────────

type model struct {
	list         list.Model
	coordinator  *device.Coordinator
	loading      bool
	errs         []error
	quitting     bool
	width        int
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

func newListStyles() list.Styles {
	s := list.DefaultStyles(true)

	s.StatusBar = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint).
		Padding(0, 0, 1, 2)
	s.StatusEmpty = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint)
	s.StatusBarActiveFilter = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorAccent2)
	s.StatusBarFilterCount = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint)
	s.NoItems = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint).
		Padding(0, 2)
	s.PaginationStyle = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint).
		PaddingLeft(2)
	s.HelpStyle = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint).
		Padding(1, 0, 0, 2)
	s.ActivePaginationDot = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorAccent).
		SetString("•")
	s.InactivePaginationDot = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorBorder).
		SetString("•")
	s.DividerDot = lipgloss.NewStyle().
		Background(colorBg).
		Foreground(colorFgFaint).
		SetString(" • ")

	// filter input: accent prompt
	s.Filter = textinput.DefaultStyles(true)
	s.Filter.Focused.Prompt = lipgloss.NewStyle().Foreground(colorAccent)
	s.Filter.Blurred.Prompt = lipgloss.NewStyle().Foreground(colorAccent)
	s.Filter.Cursor.Color = colorAccent

	return s
}

func initialModel() model {
	l := list.New([]list.Item{}, deviceDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(true)
	l.Styles = newListStyles()

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
		h, v := styleDoc.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-1) // -1 for top bar

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
			items[idx] = item{dev: dev}
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
				lipgloss.NewStyle().Foreground(colorErr).Render(
					fmt.Sprintf("⚠  %d error(s) during discovery", len(m.errs)),
				),
			)
		} else {
			statusCmd = m.list.NewStatusMessage(
				lipgloss.NewStyle().Foreground(colorOk).Render("✓  discovery complete"),
			)
		}
		return m, statusCmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) renderTopBar() string {
	if m.width == 0 {
		return ""
	}

	sep := styleFaint.Render(" · ")

	left := styleBrand.Render("simmer") +
		styleFaint.Render(" "+appVersion) +
		sep +
		lipgloss.NewStyle().Foreground(colorOk).Background(colorBg).Render("●") +
		styleDim.Render(fmt.Sprintf(" %d booted", m.bootedCount)) +
		styleFaint.Render("  ") +
		lipgloss.NewStyle().Foreground(colorIOS).Background(colorBg).Render("") +
		styleDim.Render(fmt.Sprintf(" %d iOS", m.iosCount)) +
		styleFaint.Render("  ") +
		lipgloss.NewStyle().Foreground(colorAndroid).Background(colorBg).Render("▲") +
		styleDim.Render(fmt.Sprintf(" %d Android", m.androidCount))

	var metaParts []string
	if v := m.toolVersions[device.PlatformIOS]; v != "" {
		metaParts = append(metaParts, "xcrun "+v)
	}

	if v := m.toolVersions[device.PlatformAndroid]; v != "" {
		metaParts = append(metaParts, "adb "+v)
	}

	right := styleFaint.Render(strings.Join(metaParts, " · "))

	innerWidth := m.width - 2 // account for styleTopBar Padding(0,1)
	gap := innerWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return styleTopBar.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m model) View() tea.View {
	var body string
	if m.loading && len(m.list.Items()) == 0 {
		body = "\n  " + lipgloss.NewStyle().Foreground(colorFgFaint).Render("fetching devices…")
	} else {
		body = styleDoc.Render(m.list.View())
	}

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, m.renderTopBar(), body))
	v.AltScreen = true
	v.WindowTitle = "Simmer"
	v.BackgroundColor = colorBg
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
