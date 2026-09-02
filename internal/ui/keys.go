package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// ── Footer global key map ────────────────────────────────────────────────────

// GlobalKeyMap defines the top-level bindings shown in the footer short-help bar.
type GlobalKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Pane     key.Binding
	Boot     key.Binding
	Shutdown key.Binding
	Filter   key.Binding
	Add      key.Binding
	Delete   key.Binding
	Tabs     key.Binding
	Load     key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func (k GlobalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Boot, k.Shutdown, k.Load, k.Help, k.Quit}
}

func (k GlobalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Pane},
		{k.Boot, k.Shutdown, k.Load},
		{k.Filter, k.Add, k.Delete},
		{k.Tabs, k.Help, k.Quit},
	}
}

// GlobalKeys is the footer key map singleton.
var GlobalKeys = GlobalKeyMap{
	Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// ── Sidebar key map ──────────────────────────────────────────────────────────

// SidebarKeyMap contains all bindings for the sidebar panel.
type SidebarKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	SwitchPane key.Binding
	Toggle     key.Binding
	Filter     key.Binding
	Add        key.Binding
	Delete     key.Binding
	Boot       key.Binding
	Shutdown   key.Binding
	Load       key.Binding
	Refresh    key.Binding
	Help       key.Binding
	Quit       key.Binding
}

func (k SidebarKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.SwitchPane, k.Boot, k.Load, k.Help, k.Quit}
}

func (k SidebarKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.SwitchPane},
		{k.Boot, k.Shutdown, k.Load},
		{k.Filter, k.Add, k.Delete},
		{k.Toggle, k.Refresh, k.Help, k.Quit},
	}
}

// SidebarKeys is the sidebar key map singleton.
var SidebarKeys = SidebarKeyMap{
	Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	SwitchPane: key.NewBinding(key.WithKeys("tab", "right", "l", "left", "h"), key.WithHelp("tab/←→", "switch pane")),
	Toggle:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "collapse group")),
	Filter:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	Add:        key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add device")),
	Delete:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Boot:       key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "boot")),
	Shutdown:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shutdown")),
	Load:       key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "load device")),
	Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// ── Main pane key map ────────────────────────────────────────────────────────

// MainPaneKeyMap contains all bindings for the main detail pane.
type MainPaneKeyMap struct {
	Up    key.Binding
	Down  key.Binding
	Home  key.Binding
	End   key.Binding
	Panel key.Binding
	Back  key.Binding
	Info  key.Binding
	// Apps panel
	Filter  key.Binding
	Install key.Binding
	Delete  key.Binding
	PinApp  key.Binding
	Log     key.Binding
	Rebuild key.Binding
	// Files panel
	Expand key.Binding
	Help   key.Binding
}

func (k MainPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Panel, k.Back, k.Help}
}

func (k MainPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Home, k.End},
		{k.Panel, k.Back},
		{k.Filter, k.Install, k.Delete, k.PinApp, k.Log, k.Rebuild},
		{k.Info, k.Expand, k.Help},
	}
}

// Panel-specific help views over MainPaneKeys: the "?" overlay shows only the
// keys that do something in the panel that is currently expanded.

type appsPaneHelp struct{}

func (appsPaneHelp) ShortHelp() []key.Binding {
	k := MainPaneKeys
	return []key.Binding{k.Up, k.Panel, k.Filter, k.Info, k.Help}
}

func (appsPaneHelp) FullHelp() [][]key.Binding {
	k := MainPaneKeys
	return [][]key.Binding{
		{k.Up, k.Down, k.Home, k.End},
		{k.Panel, k.Back, k.Info},
		{k.Filter, k.Install, k.Delete, k.PinApp, k.Log, k.Rebuild},
		{k.Help},
	}
}

type filesPaneHelp struct{}

func (filesPaneHelp) ShortHelp() []key.Binding {
	k := MainPaneKeys
	return []key.Binding{k.Up, k.Panel, k.Expand, k.Info, k.Help}
}

func (filesPaneHelp) FullHelp() [][]key.Binding {
	k := MainPaneKeys
	return [][]key.Binding{
		{k.Up, k.Down, k.Home, k.End},
		{k.Panel, k.Back, k.Info},
		{k.Expand},
		{k.Help},
	}
}

// MainPaneAppsKeys / MainPaneFilesKeys are the per-panel help key maps.
var (
	MainPaneAppsKeys  help.KeyMap = appsPaneHelp{}
	MainPaneFilesKeys help.KeyMap = filesPaneHelp{}
)

// MainPaneKeys is the main pane key map singleton.
var MainPaneKeys = MainPaneKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Home:    key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g/home", "top")),
	End:     key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "bottom")),
	Panel:   key.NewBinding(key.WithKeys("2", "3"), key.WithHelp("2/3", "focus apps/files")),
	Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Info:    key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "device info")),
	Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter apps")),
	Install: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "install app")),
	Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete app")),
	PinApp:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "browse app files")),
	Log:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "log app to file")),
	Rebuild: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rebuild & reinstall")),
	Expand:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand dir / open db")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
}

// ── Device-info overlay key map ────────────────────────────────────────────

// InfoOverlayKeyMap contains bindings for the device-info sheet.
type InfoOverlayKeyMap struct {
	Close  key.Binding
	Copy   key.Binding
	Reveal key.Binding
}

func (k InfoOverlayKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Close, k.Copy, k.Reveal}
}

func (k InfoOverlayKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Close, k.Copy, k.Reveal}}
}

// InfoOverlayKeys is the device-info sheet key map singleton.
var InfoOverlayKeys = InfoOverlayKeyMap{
	Close:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
	Copy:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy udid")),
	Reveal: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reveal in Finder")),
}

// ── Install app file picker key map ─────────────────────────────────────────

// InstallPickerKeyMap contains bindings for the install-app file picker modal.
type InstallPickerKeyMap struct {
	Select key.Binding
	Open   key.Binding
	Up     key.Binding
	Down   key.Binding
	Back   key.Binding
	Cancel key.Binding
}

func (k InstallPickerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Open, k.Back, k.Cancel}
}

func (k InstallPickerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Select, k.Open},
		{k.Up, k.Down, k.Back},
		{k.Cancel},
	}
}

// InstallPickerKeys is the install-app picker key map singleton.
var InstallPickerKeys = InstallPickerKeyMap{
	Select: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "select file/folder")),
	Open:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open folder")),
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Back:   key.NewBinding(key.WithKeys("h", "backspace", "left"), key.WithHelp("h/←", "go back")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

// ── DB viewer key map ────────────────────────────────────────────────────────

// DBViewerKeyMap contains all bindings for the SQLite DB viewer modal.
type DBViewerKeyMap struct {
	FocusNext    key.Binding
	FocusPrev    key.Binding
	FocusSidebar key.Binding
	FocusQuery   key.Binding
	QueryDown    key.Binding
	QueryUp      key.Binding
	Select       key.Binding
	RunQuery     key.Binding
	RunAll       key.Binding
	Save         key.Binding
	Settings     key.Binding
	Close        key.Binding
}

func (k DBViewerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.FocusNext, k.RunQuery, k.Save, k.Close}
}

func (k DBViewerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.FocusNext, k.FocusPrev, k.FocusSidebar, k.FocusQuery},
		{k.QueryDown, k.QueryUp},
		{k.Select, k.RunQuery, k.RunAll},
		{k.Save, k.Settings, k.Close},
	}
}

// DBViewerKeys is the DB viewer key map singleton.
var DBViewerKeys = DBViewerKeyMap{
	FocusNext:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
	FocusPrev:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev pane")),
	FocusSidebar: key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "go to tables")),
	FocusQuery:   key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "go to query")),
	QueryDown:    key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "query → results")),
	QueryUp:      key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("ctrl+k", "results → query")),
	Select:       key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "run table query")),
	RunQuery:     key.NewBinding(key.WithKeys("f5"), key.WithHelp("f5", "run query")),
	RunAll:       key.NewBinding(key.WithKeys("ctrl+enter"), key.WithHelp("ctrl+enter", "run all")),
	Save:         key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save query")),
	Settings:     key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s (no query)", "settings")),
	Close:        key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", "close")),
}

// ── Shared helpers ───────────────────────────────────────────────────────────

// NewHelpModel returns a help.Model styled to match the Simmer UI palette.
func NewHelpModel() help.Model {
	h := help.New()
	h.ShortSeparator = "  "
	h.FullSeparator = "    "
	h.Styles = help.Styles{
		ShortKey:       lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true).Inline(true),
		ShortDesc:      lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Inline(true),
		ShortSeparator: lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Inline(true),
		Ellipsis:       lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Inline(true),
		FullKey:        lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true),
		FullDesc:       lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg),
		FullSeparator:  lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg),
	}
	return h
}
