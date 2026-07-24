package app

import (
	"fmt"
	"strings"
	"time"

	"simmer/internal/device"
	"simmer/internal/logging"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"

	"charm.land/bubbles/v2/spinner"
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
	logger       *logging.Logger
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

	helpOverlay *ui.HelpOverlay

	platformPicker  *ui.PlatformPickerModal
	createIOSModal  *ui.CreateSimulatorModal
	createAndModal  *ui.CreateAndroidEmulatorModal
	deleteAlert     *ui.DeleteSimulatorAlert
	deleteAppAlert  *ui.DeleteAppAlert
	installAppModal *ui.InstallAppModal
	buildStream     *device.BuildStream
	buildDeviceID   string
	installing      bool
	booting         bool
	installSpinner  spinner.Model
	sqliteModal     *ui.SQLiteModal
	dbViewerModal   *dbviewer.Modal

	// rebuildPaths caches the resolved Android gradle project directory per
	// bundle-id for the running session (no persistence — see
	// docs/adr/0001-project-resolution-not-persisted.md). iOS resolves fresh
	// each time via DerivedData, so it needs no cache.
	rebuildPaths map[string]string
}

func initialModel(version string, logger *logging.Logger) model {
	coord := device.NewCoordinator(
		device.NewIOSManager(logger),
		device.NewPhysicalIOSManager(logger),
		device.NewAndroidManager(logger),
		device.NewPhysicalAndroidManager(logger),
	)

	m := model{
		sidebar:      ui.NewSidebar(),
		mainPane:     ui.NewMainPane(),
		focus:        focusSidebar,
		coordinator:  coord,
		logger:       logger,
		loading:      true,
		toolVersions: make(map[device.Platform]string),
		rebuildPaths: make(map[string]string),
		appVersion:   version,
		installSpinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(ui.ColorAccent)),
		),
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

// contextHelpOverlay returns a HelpOverlay keyed to the currently active context:
// DB viewer, main pane, or sidebar.
func (m *model) contextHelpOverlay() ui.HelpOverlay {
	if m.installAppModal != nil {
		return ui.NewHelpOverlay("Install App", ui.InstallPickerKeys)
	}
	if m.dbViewerModal != nil {
		return ui.NewHelpOverlay("DB Viewer", ui.DBViewerKeys)
	}
	if m.focus == focusMain {
		return ui.NewHelpOverlay("Main Pane", ui.MainPaneKeys)
	}
	return ui.NewHelpOverlay("Sidebar", ui.SidebarKeys)
}

const autoRefreshInterval = 30 * time.Second

func scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(_ time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m model) bodyHeight() int {
	topBar := ui.RenderTopBar(ui.TopBarParams{Width: m.width})
	footer := ui.RenderFooter(ui.FooterParams{Width: m.width})
	return max(m.height-lipgloss.Height(topBar)-lipgloss.Height(footer), 0)
}

func Run(version string, logger *logging.Logger) error {
	m := initialModel(version, logger)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run program: %w", err)
	}

	return nil
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
