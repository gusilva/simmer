package ui

import (
	"fmt"
	"strings"

	"simmer/pkg/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// MainTab identifies one of the top-level tabs in the main pane.
type MainTab int

const (
	// TabFiles shows the device filesystem tree + preview.
	TabFiles MainTab = iota
	// TabLogs shows device logs (not implemented).
	TabLogs
	// TabApps shows installed apps (not implemented).
	TabApps
	// TabInfo shows device info (not implemented).
	TabInfo
)

var mainTabs = []Tab{
	{Key: "1", Label: "Files"},
	{Key: "2", Label: "Logs"},
	{Key: "3", Label: "Apps"},
	{Key: "4", Label: "Info"},
}

// MainPane is the right-hand panel showing details for the active device.
// It is a pure UI component: callers pass in the active device and a loaded
// filesystem tree via SetDevice.
type MainPane struct {
	active   *device.Device
	tab      MainTab
	tree     *device.FileNode
	expanded map[string]bool
	treeIdx  int
	focused  bool

	width  int
	height int
}

// NewMainPane returns an empty main pane.
func NewMainPane() MainPane {
	return MainPane{expanded: map[string]bool{}}
}

// SetSize sets the outer width/height available to the pane.
func (m *MainPane) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused marks whether the main pane holds the app's outer focus.
// Affects only border styling; key handling is owned by the parent.
func (m *MainPane) SetFocused(f bool) { m.focused = f }

// SetDevice loads a device and its filesystem root into the pane. Pass nil
// for both to clear the pane.
func (m *MainPane) SetDevice(d *device.Device, root *device.FileNode) {
	m.active = d
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

// HasDevice reports whether a device is currently loaded.
func (m MainPane) HasDevice() bool { return m.active != nil }

// Update handles tab-switch keys. Other messages are ignored.
func (m MainPane) Update(msg tea.Msg) (MainPane, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "1":
		m.tab = TabFiles
	case "2":
		m.tab = TabLogs
	case "3":
		m.tab = TabApps
	case "4":
		m.tab = TabInfo
	}
	return m, nil
}

// View renders the main pane.
func (m MainPane) View() string {
	if m.width < 12 || m.height < 5 {
		return ""
	}

	borderC := ColorBorder
	if m.focused {
		borderC = ColorBorderHi
	}
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderC).
		BorderBackground(ColorBg).
		Background(ColorBg)

	innerW := m.width - 2
	innerH := m.height - 2

	if m.active == nil {
		return border.Render(m.renderHint(innerW, innerH))
	}

	title := m.renderTitleRow(innerW)
	divider := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Background(ColorBg).
		Render(strings.Repeat("─", innerW))
	tabs := RenderTabs(mainTabs, int(m.tab), innerW)

	fixedH := lipgloss.Height(title) + lipgloss.Height(divider) +
		lipgloss.Height(tabs) + lipgloss.Height(divider)
	contentH := innerH - fixedH
	if contentH < 1 {
		contentH = 1
	}
	content := m.renderTabContent(innerW, contentH)

	inner := strings.Join([]string{title, divider, tabs, divider, content}, "\n")
	return border.Render(inner)
}

// ── rendering helpers ──────────────────────────────────────────────────

func (m MainPane) renderHint(innerW, innerH int) string {
	hint := lipgloss.NewStyle().
		Foreground(ColorFgFaint).
		Background(ColorBg).
		Render("  press space on a device to load")
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
	gap := innerW - lipgloss.Width(left) - lipgloss.Width(udid) - 1
	if gap < 1 {
		gap = 1
	}
	return left + bg.Render(strings.Repeat(" ", gap)) + udid + bg.Render(" ")
}

func (m MainPane) renderTabContent(innerW, innerH int) string {
	switch m.tab {
	case TabFiles:
		return m.renderFiles(innerW, innerH)
	default:
		return m.renderPlaceholder(innerW, innerH)
	}
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
	const sepW = 1
	treeW := innerW * 52 / 100
	if treeW < 24 {
		treeW = 24
	}
	previewW := innerW - treeW - sepW
	if previewW < 16 {
		previewW = 16
		treeW = innerW - sepW - previewW
	}

	treeLines := m.renderTreePane(treeW, innerH)
	previewLines := m.renderPreviewPane(previewW, innerH)

	sep := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Background(ColorBg).
		Render("│")

	rows := make([]string, 0, innerH)
	for i := 0; i < innerH; i++ {
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
	var lines []string

	crumb := m.renderCrumb(w)
	lines = append(lines, crumb)
	lines = append(lines, padBg(w))

	rows := m.flattenTree()
	for i, r := range rows {
		lines = append(lines, m.renderTreeRow(r, w, i == m.treeIdx))
		if len(lines) >= h {
			break
		}
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	return lines
}

func (m MainPane) renderCrumb(w int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)
	root := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render("~/")
	name := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(m.active.Name)
	rest := ""
	if m.tree != nil {
		rest = lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render(" · " + m.tree.Path)
	}
	row := " " + root + name + rest
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

	icon := ""
	iconC := ColorFgDim
	if r.node.IsDir {
		icon = ""
		iconC = ColorInfo
	}

	meta := ""
	if !r.node.IsDir {
		meta = formatSize(r.node.Size)
	} else if len(r.node.Children) == 0 {
		meta = "—"
	}

	if selected {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		left := " " + indent + caretCh + " " + icon + " " + r.node.Name
		gap := w - lipgloss.Width(left) - lipgloss.Width(meta) - 1
		if gap < 1 {
			gap = 1
		}
		return sel.Render(left + strings.Repeat(" ", gap) + meta + " ")
	}

	caret := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(caretCh)
	ic := lipgloss.NewStyle().Foreground(iconC).Background(ColorBg).Render(icon)
	name := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(r.node.Name)
	metaStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(meta)

	left := " " + indent + caret + " " + ic + " " + name
	gap := w - lipgloss.Width(left) - lipgloss.Width(meta) - 1
	if gap < 1 {
		gap = 1
	}
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
		labelL := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("preview ")
		labelN := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(sel.Name)
		lines = append(lines, " "+labelL+labelN)
	}
	lines = append(lines, padBg(w))

	if sel != nil {
		fields := previewFields(sel)
		for _, f := range fields {
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
	keyW := 12
	keyText := k
	if len(keyText) > keyW {
		keyText = keyText[:keyW]
	}
	keyText = padRight(keyText, keyW)

	keyStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(keyText)
	valStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(v)

	row := " " + keyStyled + " " + valStyled
	if pad := w - lipgloss.Width(row); pad > 0 {
		row += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
	}
	return row
}

// ── small utilities ────────────────────────────────────────────────────

func padBg(n int) string {
	if n <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", n))
}

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
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

func fileExtSuffix(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return "(" + name[i:] + ")"
	}
	return ""
}
