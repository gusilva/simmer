package ui

import (
	"fmt"
	"strings"

	"simmer/internal/device"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// MainTab identifies one of the top-level tabs in the main pane.
// The iota values must stay in lockstep with the order of mainTabs.
type MainTab int

const (
	// TabInfo shows device info key/value rows.
	TabInfo MainTab = iota
	// TabApps shows installed apps.
	TabApps
	// TabLogs shows device logs (not implemented).
	TabLogs
	// TabFiles shows the device filesystem tree + preview.
	TabFiles
)

var mainTabs = []Tab{
	{Key: "1", Label: "Info"},
	{Key: "2", Label: "Apps"},
	{Key: "3", Label: "Logs"},
	{Key: "4", Label: "Files"},
}

// MainPane is the right-hand panel showing details for the active device.
// It is a pure UI component: callers pass in the active device and a loaded
// filesystem tree via SetDevice.
type MainPane struct {
	active      *device.Device    // Which device is selected (nil = none)
	tab         MainTab           // Which tab is active: Info, Apps, Logs, Files
	tree        *device.FileNode  // Filesystem tree for Files Tab
	expanded    map[string]bool   // Which dirs are open in the tree
	treeIdx     int               // Cursor row in the tree
	apps        []device.App      // List of installed apps
	appsIdx     int               // Cursor row in the apps list
	selectedApp *device.App       // The app with log streaming ON
	info        device.DeviceInfo // Device info fields for the Info tab
	infoIdx     int               // Cursor row in the info list
	logs        []string          // Buffered log lines
	logBundle   string            // Which app's logs we're streaming
	logsVP      viewport.Model    // Scrollable viewport for logs
	focused     bool              // Does this pane have keyboard focus?
	width       int               // Outer width available to the pane (including borders)
	height      int               // Outer height available to the pane (including borders)
}

// NewMainPane returns an empty main pane.
func NewMainPane() MainPane {
	vp := viewport.New() // scrollable text area
	vp.SoftWrap = true
	vp.Style = lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	return MainPane{expanded: map[string]bool{}, logsVP: vp}
}

// SetSize sets the outer width/height available to the pane and rescales
// child components (logs viewport) accordingly.
func (m *MainPane) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Logs viewport sits inside: outer frame (2) + tab header (4 lines:
	// title, divider, tabs, divider) + log header (2 lines: header + rule).
	innerW := max(w-2, 1)
	innerH := max(h-2, 1)
	contentH := max(innerH-4, 1)
	vpH := max(contentH-2, 1)
	vpW := max(innerW-2, 1)

	m.logsVP.SetWidth(vpW)
	m.logsVP.SetHeight(vpH)
}

// SetFocused marks whether the main pane holds the app's outer focus.
// Affects only border styling; key handling is owned by the parent.
func (m *MainPane) SetFocused(f bool) { m.focused = f }

// SetDevice loads a device and its filesystem root into the pane. Pass nil
// for both to clear the pane. Resets the apps list as well.
func (m *MainPane) SetDevice(d *device.Device, root *device.FileNode) {
	m.active = d
	m.tree = root
	m.expanded = map[string]bool{}
	m.treeIdx = 0
	m.apps = nil
	m.appsIdx = 0
	m.selectedApp = nil
	m.info = device.DeviceInfo{}
	m.infoIdx = 0
	m.logs = nil
	m.logBundle = ""

	if root != nil {
		m.expanded[root.Path] = true

		for _, c := range root.Children {
			if c.IsDir {
				m.expanded[c.Path] = true
			}
		}
	}
}

// SyncActiveDevice updates the active device's fields from the refreshed list.
// All other pane state (tabs, tree, apps, info, logs) is preserved.
// If the active device is no longer in the list (e.g. deleted), the pane is cleared.
func (m *MainPane) SyncActiveDevice(devs []device.Device) {
	if m.active == nil {
		return
	}
	for _, d := range devs {
		if d.ID == m.active.ID {
			*m.active = d
			for i := range m.info.Fields {
				if m.info.Fields[i].Key == "Status" {
					m.info.Fields[i].Value = string(d.Status)
					break
				}
			}
			return
		}
	}
	m.active = nil
}

// SetTree replaces only the filesystem tree without resetting the rest of the
// pane state. Use this when the tree loads asynchronously after SetDevice.
func (m *MainPane) SetTree(root *device.FileNode) {
	m.tree = root
	m.expanded = map[string]bool{}
	m.treeIdx = 0
	if root != nil {
		m.expanded[root.Path] = true
		for _, c := range root.Children {
			if c.IsDir {
				m.expanded[c.Path] = true
			}
		}
	}
}

// SetApps replaces the list of installed apps shown in the Apps tab.
func (m *MainPane) SetApps(apps []device.App) {
	m.apps = apps
	if m.appsIdx >= len(apps) {
		m.appsIdx = 0
	}
}

// SetInfo replaces the device info shown in the Info tab.
func (m *MainPane) SetInfo(info device.DeviceInfo) {
	m.info = info
	if m.infoIdx >= len(info.Fields) {
		m.infoIdx = 0
	}
}

// SelectedApp returns the app currently highlighted in the Apps tab, or nil.
func (m MainPane) SelectedApp() *device.App {
	if m.appsIdx < 0 || m.appsIdx >= len(m.apps) {
		return nil
	}
	a := m.apps[m.appsIdx]
	return &a
}

// SetLogBundle marks which app's logs are now being streamed and clears any
// previously buffered lines + viewport content.
func (m *MainPane) SetLogBundle(bundleID string) {
	m.logBundle = bundleID
	m.logs = nil
	m.logsVP.SetContent("")
	m.logsVP.GotoTop()
}

// AppendLog adds a line to the rolling log buffer. The buffer is capped at
// the most recent 1000 lines. If the viewport was at the bottom before the
// append, it scrolls to the new bottom; otherwise the user's scroll position
// is preserved.
func (m *MainPane) AppendLog(line string) {
	const max = 1000
	atBottom := m.logsVP.AtBottom()

	m.logs = append(m.logs, line)
	if len(m.logs) > max {
		m.logs = m.logs[len(m.logs)-max:]
	}
	m.logsVP.SetContent(strings.Join(m.logs, "\n"))
	if atBottom {
		m.logsVP.GotoBottom()
	}
}

// LogBundle returns the bundle identifier whose logs are currently buffered,
// or "" if none.
func (m MainPane) LogBundle() string { return m.logBundle }

// RequestLogStreamMsg is dispatched by MainPane when the user selects an app
// for log streaming. The parent program is expected to start streaming logs
// for the given app and feed them back via AppendLog.
type RequestLogStreamMsg struct {
	App device.App
}

// StopLogStreamMsg is dispatched when the user deselects the streaming app.
type StopLogStreamMsg struct{}

// RequestFileTreeMsg is dispatched when the Files tab is opened for an Android
// device that has a selected app. The parent program should load and return
// the file tree for the given app's sandbox.
type RequestFileTreeMsg struct {
	App device.App
}

// AppFocusedMsg is dispatched whenever the cursor lands on an app row in the
// Apps tab. Useful for surfacing the selection in a status bar.
type AppFocusedMsg struct {
	App device.App
}

// HasDevice reports whether a device is currently loaded.
func (m MainPane) HasDevice() bool { return m.active != nil }

// Update handles tab-switch keys plus, while on the Files tab, tree
// navigation: up/down/j/k move the cursor and enter toggles expansion of the
// selected directory.
func (m MainPane) Update(msg tea.Msg) (MainPane, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "1":
		m.tab = TabInfo
		return m, nil
	case "2":
		m.tab = TabApps
		if app := m.SelectedApp(); app != nil {
			a := *app
			return m, func() tea.Msg { return AppFocusedMsg{App: a} }
		}
		return m, nil
	case "3":
		m.tab = TabLogs
		return m, nil
	case "4":
		m.tab = TabFiles
		if m.active != nil && m.selectedApp != nil {
			app := *m.selectedApp
			return m, func() tea.Msg { return RequestFileTreeMsg{App: app} }
		}
		return m, nil
	}
	switch m.tab {
	case TabFiles:
		rows := m.flattenTree()
		switch k.String() {
		case "up", "k":
			if m.treeIdx > 0 {
				m.treeIdx--
			}
		case "down", "j":
			if m.treeIdx < len(rows)-1 {
				m.treeIdx++
			}
		case "home", "g":
			m.treeIdx = 0
		case "end", "G":
			if len(rows) > 0 {
				m.treeIdx = len(rows) - 1
			}
		case "enter":
			if m.treeIdx < 0 || m.treeIdx >= len(rows) {
				return m, nil
			}
			n := rows[m.treeIdx].node
			if !n.IsDir {
				return m, nil
			}
			m.expanded[n.Path] = !m.expanded[n.Path]
			rows = m.flattenTree()
			if m.treeIdx >= len(rows) {
				m.treeIdx = len(rows) - 1
			}
			if m.treeIdx < 0 {
				m.treeIdx = 0
			}
		}
	case TabApps:
		prev := m.appsIdx
		switch k.String() {
		case "up", "k":
			if m.appsIdx > 0 {
				m.appsIdx--
			}
		case "down", "j":
			if m.appsIdx < len(m.apps)-1 {
				m.appsIdx++
			}
		case "home", "g":
			m.appsIdx = 0
		case "end", "G":
			if len(m.apps) > 0 {
				m.appsIdx = len(m.apps) - 1
			}
		case "space":
			if len(m.apps) == 0 {
				return m, nil
			}

			app := m.apps[m.appsIdx]
			if m.selectedApp != nil && m.selectedApp.BundleID == app.BundleID {
				m.selectedApp = nil
				m.tree = nil
				return m, func() tea.Msg { return StopLogStreamMsg{} }
			}

			a := app
			m.selectedApp = &a
			m.tree = nil

			return m, func() tea.Msg { return RequestLogStreamMsg{App: a} }
		}
		if m.appsIdx != prev {
			if app := m.SelectedApp(); app != nil {
				a := *app
				return m, func() tea.Msg { return AppFocusedMsg{App: a} }
			}
		}
	case TabLogs:
		var cmd tea.Cmd
		m.logsVP, cmd = m.logsVP.Update(msg)
		return m, cmd
	case TabInfo:
		n := len(m.info.Fields)
		switch k.String() {
		case "up", "k":
			if m.infoIdx > 0 {
				m.infoIdx--
			}
		case "down", "j":
			if m.infoIdx < n-1 {
				m.infoIdx++
			}
		case "home", "g":
			m.infoIdx = 0
		case "end", "G":
			if n > 0 {
				m.infoIdx = n - 1
			}
		case "space":
			if m.infoIdx < 0 || m.infoIdx >= n {
				return m, nil
			}
			return m, CopyToClipboardCmd(m.info.Fields[m.infoIdx].Value)
		}
	}
	return m, nil
}

// View renders the main pane with a manually composed frame so the inner
// divider rules can tee (┬/┴) into the vertical Files separator.
func (m MainPane) View() string {
	if m.width < 12 || m.height < 5 {
		return ""
	}

	borderC := ColorBorder
	if m.focused {
		borderC = ColorBorderHi
	}
	frame := lipgloss.NewStyle().Foreground(borderC).Background(ColorBg)
	innerRule := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)

	innerW := m.width - 2
	innerH := m.height - 2

	treeW, _, _ := filesLayout(innerW)

	rows := make([]string, 0, innerH)
	if m.active == nil {
		rows = append(rows, strings.Split(m.renderHint(innerW, innerH), "\n")...)
	} else {
		rows = append(rows, m.renderTitleRow(innerW))
		rows = append(rows, hrule(innerW, -1, "", innerRule))
		rows = append(rows, RenderTabs(mainTabs, int(m.tab), innerW))
		junction := -1
		if m.tab == TabFiles && m.tree != nil {
			junction = treeW
		}
		rows = append(rows, hrule(innerW, junction, "┬", innerRule))

		contentH := max(innerH-len(rows), 1)
		rows = append(rows, strings.Split(m.renderTabContent(innerW, contentH), "\n")...)
	}

	if len(rows) > innerH {
		rows = rows[:innerH]
	}
	for len(rows) < innerH {
		rows = append(rows, padBg(innerW))
	}

	side := frame.Render("│")
	wrapped := make([]string, 0, innerH+2)
	wrapped = append(wrapped, frame.Render("╭"+strings.Repeat("─", innerW)+"╮"))
	for _, r := range rows {
		wrapped = append(wrapped, side+r+side)
	}

	bottomDashes := strings.Repeat("─", innerW)
	if m.active != nil && m.tab == TabFiles && m.tree != nil && treeW > 0 && treeW < innerW {
		bottomDashes = strings.Repeat("─", treeW) + "┴" + strings.Repeat("─", innerW-treeW-1)
	}
	wrapped = append(wrapped, frame.Render("╰"+bottomDashes+"╯"))

	return strings.Join(wrapped, "\n")
}

// hrule renders a horizontal rule of `width` cells, optionally inserting a
// junction glyph at column `at`. Set at < 0 (or junction == "") for a plain rule.
func hrule(width, at int, junction string, style lipgloss.Style) string {
	if at < 0 || at >= width || junction == "" {
		return style.Render(strings.Repeat("─", width))
	}
	return style.Render(strings.Repeat("─", at) + junction + strings.Repeat("─", width-at-1))
}

// filesLayout returns the column split used by the Files tab.
func filesLayout(innerW int) (treeW, sepW, previewW int) {
	sepW = 1
	treeW = max(innerW*52/100, 24)
	previewW = innerW - treeW - sepW
	if previewW < 16 {
		previewW = 16
		treeW = innerW - sepW - previewW
	}
	return
}

// ── rendering helpers ──────────────────────────────────────────────────

func (m MainPane) renderHint(innerW, innerH int) string {
	hint := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("  press space on a device to load")

	if pad := innerW - lipgloss.Width(hint); pad > 0 {
		hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
	}

	lines := []string{hint}
	for len(lines) < innerH {
		lines = append(lines, padBg(innerW))
	}
	return strings.Join(lines, "\n")
}

func (m MainPane) renderTitleRow(innerW int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	dotC := ColorFgFaint
	if m.active.Status == device.StatusRunning {
		dotC = ColorOk
	}
	dot := lipgloss.NewStyle().Foreground(dotC).Background(ColorBg).Render("●")
	name := lipgloss.NewStyle().
		Foreground(ColorBorderHi).
		Background(ColorBg).
		Bold(true).
		Render(m.active.Name)
	sep := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("·")
	osText := fmt.Sprintf("%s %s", m.active.Platform, m.active.Version)
	osLabel := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render(osText)

	udidShort := m.active.ID
	if len(udidShort) > 8 {
		udidShort = udidShort[:8] + "…"
	}
	udid := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("udid " + udidShort)

	left := " " + dot + " " + name + " " + sep + " " + osLabel
	gap := max(innerW-lipgloss.Width(left)-lipgloss.Width(udid)-1, 1)
	return left + bg.Render(strings.Repeat(" ", gap)) + udid + bg.Render(" ")
}

func (m MainPane) renderTabContent(innerW, innerH int) string {
	switch m.tab {
	case TabFiles:
		return m.renderFiles(innerW, innerH)
	case TabApps:
		return m.renderApps(innerW, innerH)
	case TabInfo:
		return m.renderInfo(innerW, innerH)
	case TabLogs:
		return m.renderLogs(innerW, innerH)
	default:
		return m.renderPlaceholder(innerW, innerH)
	}
}

// ── Info tab ───────────────────────────────────────────────────────────

func (m MainPane) renderInfo(w, h int) string {

	if len(m.info.Fields) == 0 {
		hint := lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("  loading info…")

		if pad := w - lipgloss.Width(hint); pad > 0 {
			hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}

		lines := []string{hint}
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}

		return strings.Join(lines, "\n")
	}

	visibleH := h
	if visibleH < 1 {
		visibleH = 1
	}
	offset := 0
	if m.infoIdx >= visibleH {
		offset = m.infoIdx - visibleH + 1
	}
	end := offset + visibleH
	if end > len(m.info.Fields) {
		end = len(m.info.Fields)
	}

	keyW := infoKeyWidth(m.info.Fields)

	lines := make([]string, 0, h)
	for i := offset; i < end; i++ {
		f := m.info.Fields[i]
		lines = append(lines, renderInfoRow(f.Key, f.Value, keyW, w, i == m.infoIdx))
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	return strings.Join(lines, "\n")
}

// infoKeyWidth picks the column width for keys: longest visible key, capped at
// 14 so a single long label doesn't squeeze the value column.
func infoKeyWidth(fields []device.InfoField) int {
	const cap_ = 14
	w := 0
	for _, f := range fields {
		if kw := lipgloss.Width(f.Key); kw > w {
			w = kw
		}
	}
	if w > cap_ {
		w = cap_
	}
	if w < 4 {
		w = 4
	}
	return w
}

func renderInfoRow(k, v string, keyW, w int, selected bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	keyText := k
	if lipgloss.Width(keyText) > keyW {
		keyText = truncateName(keyText, keyW)
	}
	keyText = padRight(keyText, keyW)

	const leadW = 1
	const sepW = 1
	const trailW = 1
	valBudget := max(w-leadW-keyW-sepW-trailW, 1)
	if lipgloss.Width(v) > valBudget {
		v = truncateName(v, valBudget)
	}

	if selected {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		return sel.Render(" " + keyText + " " + v + strings.Repeat(" ", valBudget-lipgloss.Width(v)) + " ")
	}

	keyStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(keyText)
	valStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(v)
	row := " " + keyStyled + " " + valStyled
	if pad := w - lipgloss.Width(row) - trailW; pad > 0 {
		row += bg.Render(strings.Repeat(" ", pad))
	}
	return row + bg.Render(" ")
}

// ── Logs tab ───────────────────────────────────────────────────────────

func (m MainPane) renderLogs(w, h int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	if m.logBundle == "" {
		rendered := lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("  no selected app — press space on an app to start streaming")
		if pad := w - lipgloss.Width(rendered); pad > 0 {
			rendered += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines := []string{rendered}
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return strings.Join(lines, "\n")
	}

	headerL := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("  streaming ")
	headerN := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(m.logBundle)
	header := headerL + headerN
	rule := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Background(ColorBg).
		Render(strings.Repeat("─", w))

	lines := []string{header, rule}

	for _, vpLine := range strings.Split(m.logsVP.View(), "\n") {
		lines = append(lines, " "+vpLine)
	}

	for i, line := range lines {
		if pad := w - lipgloss.Width(line); pad > 0 {
			lines[i] = line + bg.Render(strings.Repeat(" ", pad))
		}
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	return strings.Join(lines, "\n")
}

// ── Apps tab ───────────────────────────────────────────────────────────

func (m MainPane) renderApps(w, h int) string {
	if len(m.apps) == 0 {
		hint := lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("  no apps installed")

		if pad := w - lipgloss.Width(hint); pad > 0 {
			hint += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}

		lines := []string{hint}
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}

		return strings.Join(lines, "\n")
	}

	visibleH := max(h, 1)
	offset := 0
	if m.appsIdx >= visibleH {
		offset = m.appsIdx - visibleH + 1
	}

	end := min(offset+visibleH, len(m.apps))
	lines := make([]string, 0, h)
	for i := offset; i < end; i++ {
		streaming := m.selectedApp != nil && m.selectedApp.BundleID == m.apps[i].BundleID
		lines = append(lines, m.renderAppRow(m.apps[i], w, i == m.appsIdx, streaming))
	}

	for len(lines) < h {
		lines = append(lines, padBg(w))
	}

	return strings.Join(lines, "\n")
}

func (m MainPane) renderAppRow(app device.App, w int, cursor bool, streaming bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	label := app.Label()
	meta := app.ShortVersion
	if meta == "" {
		meta = app.Version
	}

	// ⊙ = broadcast/signal indicator; 2 cells: space + glyph
	const streamIcon = " ⊙"
	const streamIconW = 2

	iconStr := ""
	iconW := 0
	if streaming {
		iconStr = streamIcon
		iconW = streamIconW
	}

	const (
		leadW  = 1
		trailW = 1
		minGap = 2
	)

	metaW := lipgloss.Width(meta)
	nameMax := max(w-leadW-iconW-metaW-trailW-minGap, 1)
	nameStr := truncateName(label, nameMax)
	gap := max(w-leadW-lipgloss.Width(nameStr)-iconW-metaW-trailW, minGap)

	if cursor {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		return sel.Render(" " + nameStr + iconStr + strings.Repeat(" ", gap) + meta + " ")
	}

	nameStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(nameStr)
	metaStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(meta)

	if streaming {
		iconStyled := lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Render(iconStr)
		return " " + nameStyled + iconStyled + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
	}

	return " " + nameStyled + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
}

func (m MainPane) renderPlaceholder(innerW, innerH int) string {
	body := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("  (not implemented yet)")
	lines := []string{body}
	for len(lines) < innerH {
		lines = append(lines, padBg(innerW))
	}
	return strings.Join(lines, "\n")
}

func (m MainPane) renderFiles(innerW, innerH int) string {
	if m.tree == nil {
		lines := m.renderTreePane(innerW, innerH)
		return strings.Join(lines, "\n")
	}

	treeW, _, previewW := filesLayout(innerW)

	treeLines := m.renderTreePane(treeW, innerH)
	previewLines := m.renderPreviewPane(previewW, innerH)

	sep := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Background(ColorBg).
		Render("│")

	rows := make([]string, 0, innerH)
	for i := range innerH {
		t := padBg(treeW)
		p := padBg(previewW)
		if i < len(treeLines) {
			t = treeLines[i]
		}
		if i < len(previewLines) {
			p = previewLines[i]
		}
		rows = append(rows, t+sep+p)
	}
	return strings.Join(rows, "\n")
}

// ── Files: tree pane ───────────────────────────────────────────────────

type treeRow struct {
	node  *device.FileNode
	depth int
}

func (m MainPane) flattenTree() []treeRow {
	var out []treeRow
	if m.tree == nil {
		return out
	}
	var walk func(n *device.FileNode, depth int)
	walk = func(n *device.FileNode, depth int) {
		out = append(out, treeRow{node: n, depth: depth})
		if !n.IsDir || !m.expanded[n.Path] {
			return
		}
		for i := range n.Children {
			walk(&n.Children[i], depth+1)
		}
	}
	for i := range m.tree.Children {
		walk(&m.tree.Children[i], 0)
	}
	return out
}

func (m MainPane) renderTreePane(w, h int) []string {
	lines := []string{m.renderCrumb(w), padBg(w)}

	if m.tree == nil && m.active != nil {
		var hint string
		if m.selectedApp == nil {
			hint = "  select an app in the Apps tab to browse its files"
		} else {
			hint = "  loading files…"
		}
		rendered := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(hint)
		pad := w - lipgloss.Width(rendered)
		if pad > 0 {
			rendered += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, rendered)
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return lines
	}

	visibleH := max(h-len(lines), 1)
	rows := m.flattenTree()

	// Window the slice so the cursor is always visible.
	offset := 0
	if m.treeIdx >= visibleH {
		offset = m.treeIdx - visibleH + 1
	}
	end := min(offset+visibleH, len(rows))

	for i := offset; i < end; i++ {
		lines = append(lines, m.renderTreeRow(rows[i], w, i == m.treeIdx))
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	return lines
}

func (m MainPane) renderCrumb(w int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	const lead = " "
	const rootPrefix = "~/"
	const sep = " · "

	plainName := m.active.Name
	plainPath := ""
	if m.tree != nil {
		plainPath = m.tree.Path
	}

	// Budget the plain text to fit `w` cells. Trim path first, then name.
	avail := w - len(lead) - len(rootPrefix)
	if avail < 1 {
		return bg.Render(strings.Repeat(" ", w))
	}
	if plainPath != "" {
		if budget := avail - lipgloss.Width(plainName) - len(sep); budget > 0 {
			plainPath = truncateName(plainPath, budget)
		} else {
			plainPath = ""
		}
	}
	if lipgloss.Width(plainName)+len(sep)+lipgloss.Width(plainPath) > avail {
		nameBudget := avail - len(sep) - lipgloss.Width(plainPath)
		if nameBudget < 1 {
			nameBudget = avail
			plainPath = ""
		}
		plainName = truncateName(plainName, nameBudget)
	}

	root := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render(rootPrefix)
	name := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(plainName)
	rest := ""
	if plainPath != "" {
		rest = lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render(sep + plainPath)
	}
	row := lead + root + name + rest
	if pad := w - lipgloss.Width(row); pad > 0 {
		row += bg.Render(strings.Repeat(" ", pad))
	}
	return row
}

func (m MainPane) renderTreeRow(r treeRow, w int, selected bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	indent := strings.Repeat("  ", r.depth)

	caretCh := " "
	if r.node.IsDir {
		if m.expanded[r.node.Path] {
			caretCh = "▾"
		} else {
			caretCh = "▸"
		}
	}

	meta := ""
	if !r.node.IsDir {
		meta = formatSize(r.node.Size)
	} else if len(r.node.Children) == 0 {
		meta = "—"
	}

	// Layout: " " + indent + caret(1) + " " + name + GAP + meta + " "
	prefixW := 1 + lipgloss.Width(indent) + 1 + 1
	metaW := lipgloss.Width(meta)
	const minGap = 1
	const trailW = 1
	nameMax := max(w-prefixW-metaW-trailW-minGap, 1)
	nameStr := truncateName(r.node.Name, nameMax)
	nameW := lipgloss.Width(nameStr)
	gap := max(w-prefixW-nameW-metaW-trailW, minGap)

	if selected {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		return sel.Render(" " + indent + caretCh + " " + nameStr +
			strings.Repeat(" ", gap) + meta + " ")
	}

	nameC := ColorFg
	if r.node.IsDir {
		nameC = ColorInfo
	}
	caret := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(caretCh)
	name := lipgloss.NewStyle().Foreground(nameC).Background(ColorBg).Render(nameStr)
	metaStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(meta)

	left := " " + indent + caret + " " + name
	return left + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
}

// ── Files: preview pane ────────────────────────────────────────────────

func (m MainPane) renderPreviewPane(w, h int) []string {
	rows := m.flattenTree()
	var sel *device.FileNode
	if m.treeIdx >= 0 && m.treeIdx < len(rows) {
		sel = rows[m.treeIdx].node
	}

	bg := lipgloss.NewStyle().Background(ColorBg)
	var lines []string

	if sel == nil {
		lines = append(lines, " "+lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("preview"))
	} else {
		const labelText = "preview "
		nameBudget := w - 1 - len(labelText)
		nameStr := truncateName(sel.Name, nameBudget)
		labelL := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(labelText)
		labelN := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(nameStr)
		lines = append(lines, " "+labelL+labelN)
	}
	lines = append(lines, padBg(w))

	if sel != nil {
		for _, f := range previewFields(sel) {
			lines = append(lines, renderField(f.k, f.v, w))
		}
		lines = append(lines, padBg(w))
		lines = append(lines, " "+lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("── content ──"))
		for _, body := range previewContent(sel) {
			lines = append(lines, " "+lipgloss.NewStyle().
				Foreground(ColorFgDim).
				Background(ColorBg).
				Render(body))
		}
	}

	for i, line := range lines {
		if pad := w - lipgloss.Width(line); pad > 0 {
			lines[i] = line + bg.Render(strings.Repeat(" ", pad))
		}
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

type kv struct{ k, v string }

func previewFields(n *device.FileNode) []kv {
	if n.IsDir {
		return []kv{
			{"type", "directory"},
			{"items", fmt.Sprintf("%d", len(n.Children))},
			{"modified", n.Modified.Format("2006-01-02 15:04:05")},
			{"permissions", n.Permissions},
		}
	}
	return []kv{
		{"type", "file " + fileExtSuffix(n.Name)},
		{"size", formatSize(n.Size)},
		{"modified", n.Modified.Format("2006-01-02 15:04:05")},
		{"permissions", n.Permissions},
	}
}

func previewContent(n *device.FileNode) []string {
	if n.IsDir {
		return []string{fmt.Sprintf("(directory — %d entries)", len(n.Children))}
	}
	if strings.HasSuffix(n.Name, ".sqlite") {
		return []string{
			"SQLite format 3",
			fmt.Sprintf("(binary database — %s)", formatSize(n.Size)),
			"",
			"tables:  cache_entries, sync_state, attachments, ...",
			"rows:    1,284 across 14 tables",
			"wal:     enabled (38 KB)",
		}
	}
	if strings.HasSuffix(n.Name, ".log") || strings.HasSuffix(n.Name, ".json") {
		return []string{fmt.Sprintf("(text — %s)", formatSize(n.Size))}
	}
	return []string{fmt.Sprintf("(binary — %s)", formatSize(n.Size))}
}

func renderField(k, v string, w int) string {
	const keyW = 12
	const trailW = 1
	keyText := k
	if len(keyText) > keyW {
		keyText = keyText[:keyW]
	}
	keyText = padRight(keyText, keyW)

	valBudget := max(w-1-keyW-1-trailW, 1)
	if lipgloss.Width(v) > valBudget {
		v = truncateName(v, valBudget)
	}

	keyStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(keyText)
	valStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(v)

	row := " " + keyStyled + " " + valStyled
	if pad := w - lipgloss.Width(row); pad > 0 {
		row += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
	}
	return row
}

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func fileExtSuffix(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return "(" + name[i:] + ")"
	}
	return ""
}

// ── small utilities ────────────────────────────────────────────────────

func padBg(n int) string {
	if n <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", n))
}

func formatSize(b int64) string {
	switch {
	case b >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB", float64(b)/1_000_000_000)
	case b >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	case b >= 1_000:
		return fmt.Sprintf("%d KB", b/1_000)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
