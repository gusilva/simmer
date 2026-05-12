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
	Up   key.Binding
	Down key.Binding
	Home key.Binding
	End  key.Binding
	Tab  key.Binding
	Back key.Binding
	// Apps tab
	Filter key.Binding
	Stream key.Binding
	// Info tab
	Copy key.Binding
	// Files tab
	Expand key.Binding
	Help   key.Binding
}

func (k MainPaneKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Tab, k.Back, k.Help}
}

func (k MainPaneKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Home, k.End},
		{k.Tab, k.Back},
		{k.Filter, k.Stream, k.Copy},
		{k.Expand, k.Help},
	}
}

// MainPaneKeys is the main pane key map singleton.
var MainPaneKeys = MainPaneKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Home:   key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g/home", "top")),
	End:    key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G/end", "bottom")),
	Tab:    key.NewBinding(key.WithKeys("1", "2", "3", "4"), key.WithHelp("1-4", "switch tab")),
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to sidebar")),
	Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter apps")),
	Stream: key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "stream logs (apps)")),
	Copy:   key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "copy value (info)")),
	Expand: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand dir (files)")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
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
