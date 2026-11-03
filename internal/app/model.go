package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"simmer/internal/device"
	"simmer/internal/logging"
	"simmer/internal/session"
	"simmer/internal/ui"
	"simmer/internal/ui/dbviewer"
	"simmer/internal/viewstate"

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
	layout       *appLayout
	sess         *session.Session
	coordinator  *device.Coordinator
	logger       *logging.Logger
	loading      bool
	errs         []error
	quitting     bool
	width        int
	height       int
	toolVersions map[device.Platform]string
	adbServer    device.AdbServerState
	bootedCount  int
	iosCount     int
	androidCount int
	lastRefresh  time.Time
	appVersion   string
	launchDir    string

	// stopAppEnrich abandons the in-flight app-enrichment pool, if any.
	stopAppEnrich func()

	logStream   *device.LogStream
	logBundleID string
	logDeviceID string
	logFile     *os.File

	status     string
	statusKind ui.StatusKind
	statusSeq  int

	helpOverlay   *ui.HelpOverlay
	infoOverlay   *ui.InfoOverlay
	doctorOverlay *ui.DoctorOverlay

	platformPicker  *ui.PlatformPickerModal
	createIOSModal  *ui.CreateSimulatorModal
	createAndModal  *ui.CreateAndroidEmulatorModal
	deleteAlert     *ui.DeleteSimulatorAlert
	deleteAppAlert  *ui.DeleteAppAlert
	closeAppAlert   *ui.CloseAppAlert
	installAppModal *ui.InstallAppModal
	buildStream     *device.BuildStream
	buildDeviceID   string
	installing      bool
	booting         bool
	installSpinner  spinner.Model
	dbViewerModal   *dbviewer.Modal
	actionMenu      *ui.ActionMenuModal
	rnOptionsModal  *ui.RNOptionsModal
	textPromptModal *ui.TextPromptModal

	// lastClick tracks the most recent mouse click's target identity, for
	// double-click detection (see registerClick in mouse.go).
	lastClick lastClickInfo

	// rebuildPaths caches the resolved Android gradle project directory per
	// bundle-id for the running session. iOS resolves fresh
	// each time via DerivedData, so it needs no cache.
	rebuildPaths map[string]string

	// bundlerHostPorts remembers the Metro host:port last configured per
	// bundle-id via "Configure Bundler Location". On physical iOS this is
	// load-bearing, not just a convenience: RCT_jsLocation there is only ever
	// set via a one-shot launch argument (there's no `defaults write`
	// equivalent for real hardware), so it has no persistence of its own —
	// any later relaunch we trigger (e.g. "Log to File") has to re-supply it
	// or the app reverts to its compiled-in default and never reaches Metro.
	bundlerHostPorts map[string]string

	// views is this Simmer's handle on the View state; nil disables it
	// (tests, or a state directory that cannot be used).
	views *viewstate.Store
	// live reports whether this Simmer is the Active instance.
	live bool
	// recorded is the last View state handed to the debounced save.
	recorded viewstate.View
	saveSeq  int
	// restore is the resume still in progress; nil once done or abandoned.
	restore *restorePlan
	// startupNotice is a warning from claiming the View state, shown once
	// the program runs.
	startupNotice string
}

func initialModel(version string, logger *logging.Logger, launchDir string) model {
	// The Session owns device orchestration, deadlines and configuration;
	// the TUI is one adapter over it. Verbs not yet migrated still reach the
	// coordinator directly, which is the same one the Session uses.
	sess, cfgErr := session.NewDefault(logger)
	coord := sess.Coordinator()
	if cfgErr != nil {
		logger.LogError("load config", cfgErr)
	}

	m := model{
		sidebar:          ui.NewSidebar(),
		mainPane:         ui.NewMainPane(),
		focus:            focusSidebar,
		layout:           &appLayout{},
		sess:             sess,
		coordinator:      coord,
		logger:           logger,
		loading:          true,
		toolVersions:     make(map[device.Platform]string),
		rebuildPaths:     make(map[string]string),
		bundlerHostPorts: make(map[string]string),
		appVersion:       version,
		launchDir:        launchDir,
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
	if m.doctorOverlay != nil {
		return ui.NewHelpOverlay("Doctor", ui.DoctorOverlayKeys)
	}
	if m.infoOverlay != nil {
		return ui.NewHelpOverlay("Device Info", ui.InfoOverlayKeys)
	}
	if m.rnOptionsModal != nil || m.textPromptModal != nil {
		return ui.NewHelpOverlay("RN Debugging", ui.RNOptionsKeys)
	}
	if m.focus == focusMain {
		title, keys := m.mainPane.HelpKeyMap()
		return ui.NewHelpOverlay(title, keys)
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

// Options are the TUI's launch choices.
type Options struct {
	// Fresh starts from an empty View state instead of resuming the last
	// one. It still takes over.
	Fresh bool
}

func Run(version string, logger *logging.Logger, launchDir string, opts Options) error {
	m := initialModel(version, logger, launchDir)
	m.claimViewState(opts.Fresh)
	p := tea.NewProgram(m)

	final, err := p.Run()

	// The last change may still be inside its debounce window.
	if fm, ok := final.(model); ok && fm.recording() {
		if _, serr := fm.views.Save(fm.snapshot()); serr != nil {
			logger.LogError("save view state", serr)
		}
	}

	// Catch-all flush + close of any in-flight app-log file for the paths that
	// return a model: clean quit and a panic inside a Cmd. (A panic inside
	// Update/View returns a nil model — there the per-batch fsync in the
	// logBatchMsg handler is what guarantees the file is intact, and the OS
	// closes the fd on exit.) closeLogFile is idempotent, so the deterministic
	// close already done on the quit-key path is harmless here.
	if fm, ok := final.(model); ok {
		fm.closeLogFile()
		fm.stopEnrichment()
	}

	if err != nil {
		return fmt.Errorf("run program: %w", err)
	}

	return nil
}

// claimViewState takes over as the Active instance and queues the resume.
// Any failure leaves the TUI working without View state rather than not
// starting.
func (m *model) claimViewState(fresh bool) {
	dir, err := viewstate.DefaultDir()
	if err != nil {
		m.logger.LogError("view state dir", err)
		return
	}
	store := viewstate.Open(dir)
	view, err := store.Claim(fresh)
	if err != nil {
		m.logger.LogError("claim view state", err)
		if !errors.Is(err, viewstate.ErrUnreadable) {
			return
		}
		m.startupNotice = "last view could not be read; starting fresh"
	}
	m.views = store
	m.live = true
	m.recorded = view
	if view.DeviceID != "" {
		m.restore = &restorePlan{view: view, stage: restoreAwaitDevices}
	}
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

// stopEnrichment abandons any in-flight app-enrichment pool. Safe to call
// when none is running.
func (m *model) stopEnrichment() {
	if m.stopAppEnrich != nil {
		m.stopAppEnrich()
		m.stopAppEnrich = nil
	}
}
