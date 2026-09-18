package ui

import (
	"fmt"
	"image/color"
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// SidebarPane identifies which sub-panel of the sidebar holds focus.
type SidebarPane int

const (
	// PaneBooted is the top "Online" panel (running devices).
	PaneBooted SidebarPane = iota
	// PaneAvailable is the bottom "Offline" panel (stopped virtual devices).
	PaneAvailable
)

// offlineTab selects the platform tab shown in the Offline box.
type offlineTab int

const (
	tabIOS     offlineTab = 0
	tabAndroid offlineTab = 1
)

// DefaultSidebarWidth is the column width used when none is set.
const DefaultSidebarWidth = 36

// Sidebar is the left navigation panel. It is a pure UI component:
// callers feed it a flat []device.Device via SetDevices and the sidebar
// handles grouping, selection, and rendering.
type Sidebar struct {
	// Online box: all StatusRunning devices (any Kind, any Platform).
	booted []device.Device
	// Offline box: stopped virtual devices, split by platform.
	virtIOS []device.Device
	virtAnd []device.Device

	focused      SidebarPane
	outerFocused bool

	// Online box cursor and group collapse state.
	bootedIdx        int
	iosCollapsed     bool
	androidCollapsed bool

	// Offline box cursor and active platform tab.
	availIdx        int
	offlinePlatform offlineTab

	filterMode  bool
	filterQuery string

	width  int
	height int
}

// NewSidebar returns a sidebar with focus on the online panel.
func NewSidebar() Sidebar {
	return Sidebar{focused: PaneBooted, outerFocused: true, width: DefaultSidebarWidth}
}

// SetFocused marks whether the sidebar holds the app's outer focus.
func (s *Sidebar) SetFocused(f bool) { s.outerFocused = f }

// SetSize sets the sidebar's outer width and total height.
func (s *Sidebar) SetSize(w, h int) {
	if w > 0 {
		s.width = w
	}
	s.height = h
}

// Width returns the sidebar's outer width in columns.
func (s Sidebar) Width() int { return s.width }

// SetDevices replaces the device list using the routing rules:
//   - StatusRunning (any Kind, Platform) → booted (Online box)
//   - StatusOff + KindPhysical           → skip
//   - StatusOff + PlatformIOS            → virtIOS (Offline box, iOS tab)
//   - StatusOff + PlatformAndroid        → virtAnd (Offline box, Android tab)
func (s *Sidebar) SetDevices(devs []device.Device) {
	s.booted = s.booted[:0]
	s.virtIOS = s.virtIOS[:0]
	s.virtAnd = s.virtAnd[:0]
	for _, d := range devs {
		if d.Status == device.StatusRunning {
			s.booted = append(s.booted, d)
			continue
		}
		if d.Kind == device.KindPhysical {
			continue
		}
		switch d.Platform {
		case device.PlatformIOS:
			s.virtIOS = append(s.virtIOS, d)
		case device.PlatformAndroid:
			s.virtAnd = append(s.virtAnd, d)
		}
	}
	s.bootedIdx = clampIdx(s.bootedIdx, len(s.onlinePositions()))
	s.availIdx = clampIdx(s.availIdx, len(s.offlineDevices()))
}

// Devices returns all devices currently loaded in the sidebar.
func (s Sidebar) Devices() []device.Device {
	out := make([]device.Device, 0, len(s.booted)+len(s.virtIOS)+len(s.virtAnd))
	out = append(out, s.booted...)
	out = append(out, s.virtIOS...)
	out = append(out, s.virtAnd...)
	return out
}

// SelectedDevice returns the device under the cursor in the focused pane,
// or nil if the position is a group header or the pane is empty.
func (s Sidebar) SelectedDevice() *device.Device {
	switch s.focused {
	case PaneBooted:
		return s.onlineDeviceAt(s.bootedIdx)
	case PaneAvailable:
		devs := s.offlineDevices()
		if s.availIdx < len(devs) {
			d := devs[s.availIdx]
			return &d
		}
	}
	return nil
}

// FocusedPane returns which panel currently holds focus.
func (s Sidebar) FocusedPane() SidebarPane { return s.focused }

// Update handles navigation key events. Other messages are ignored.
func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	// Filter mode: intercept all keys for text input.
	if s.filterMode {
		switch k.String() {
		case "esc":
			s.filterMode = false
			s.filterQuery = ""
			s.availIdx = 0
		case "enter":
			s.filterMode = false
		case "backspace", "ctrl+h":
			if len(s.filterQuery) > 0 {
				runes := []rune(s.filterQuery)
				s.filterQuery = string(runes[:len(runes)-1])
				s.availIdx = 0
			}
		default:
			if k.Text != "" && !k.Mod.Contains(tea.ModCtrl) && !k.Mod.Contains(tea.ModAlt) {
				s.filterQuery += k.Text
				s.availIdx = 0
			}
		}
		return s, nil
	}

	switch k.String() {
	case "1":
		s.focused = PaneBooted
	case "2":
		s.focused = PaneAvailable
	case "tab", "right", "l", "left", "h", "shift+tab":
		if s.focused == PaneBooted {
			s.focused = PaneAvailable
		} else {
			s.focused = PaneBooted
		}
	case "up", "k":
		s.moveCursor(-1)
	case "down", "j":
		s.moveCursor(1)
	case "enter":
		if s.focused == PaneBooted {
			s.toggleOnlineGroup()
		}
	case "[":
		if s.focused == PaneAvailable {
			s.offlinePlatform = tabIOS
			s.availIdx = 0
		}
	case "]":
		if s.focused == PaneAvailable {
			s.offlinePlatform = tabAndroid
			s.availIdx = 0
		}
	case "esc":
		if s.focused == PaneAvailable && s.filterQuery != "" {
			s.filterQuery = ""
			s.availIdx = 0
		}
	case "f":
		if s.focused == PaneAvailable {
			s.filterMode = true
		}
	case "a":
		if s.focused == PaneAvailable {
			return s, func() tea.Msg { return ShowPlatformPickerMsg{} }
		}
	case "d":
		if s.focused == PaneAvailable {
			devs := s.offlineDevices()
			if s.availIdx < len(devs) {
				d := devs[s.availIdx]
				return s, func() tea.Msg { return ShowDeleteSimulatorMsg{Device: d} }
			}
		}
	}
	return s, nil
}

// onlineRows maps each rendered row of the Online box to its index in
// onlinePositions(), or -1 for a row that isn't a cursor stop (the blank
// separator renderOnlineRows inserts between the iOS and Android groups
// when both are present, or the "(none)" row when booted is empty). It
// mirrors renderOnlineRows' loop exactly so HandleClick's row count never
// drifts from what's actually drawn.
func (s Sidebar) onlineRows() []int {
	positions := s.onlinePositions()
	if len(positions) == 0 {
		return []int{-1}
	}
	rows := make([]int, 0, len(positions)+1)
	prevPlatform := device.Platform("")
	for i, p := range positions {
		if p.isHeader && prevPlatform != "" && prevPlatform != p.platform {
			rows = append(rows, -1)
		}
		rows = append(rows, i)
		if p.isHeader {
			prevPlatform = p.platform
		}
	}
	return rows
}

// HandleClick maps a click at row y — local to this sidebar's own View()
// output (row 0 = the Online box's top border) — to a focus and cursor
// change. Row math mirrors View() exactly, via the same helpers
// (onlineRows, offlineDevices) it renders from, so a click always lands on
// the row it visually looks like it hit.
func (s *Sidebar) HandleClick(y int) {
	rowToPos := s.onlineRows()
	onlineContentH := len(rowToPos)
	onlineBoxH := onlineContentH + 2 // top border + content + bottom border

	if y < onlineBoxH {
		s.focused = PaneBooted
		if y >= 1 && y <= onlineContentH {
			if posIdx := rowToPos[y-1]; posIdx >= 0 {
				s.bootedIdx = posIdx
			}
		}
		return
	}

	const gapLines = 1
	offTop := onlineBoxH + gapLines
	if y < offTop {
		return // blank gap row between the two boxes
	}

	offlineHeight := 0
	if s.height > 0 {
		offlineHeight = max(s.height-onlineBoxH-gapLines, 3)
	}
	localY := y - offTop
	if localY >= offlineHeight {
		return // below the offline box entirely
	}
	s.focused = PaneAvailable

	innerH := max(offlineHeight-2, 0)
	contentH := innerH
	if s.filterMode || s.filterQuery != "" {
		contentH = max(innerH-1, 0)
	}
	if localY >= 1 && localY <= contentH {
		devs := s.offlineDevices()
		if idx := localY - 1; idx < len(devs) {
			s.availIdx = idx
		}
	}
}

// View renders the sidebar. The Online box auto-fits its content; the Offline
// box fills the remaining height when SetSize was called with a positive height.
func (s Sidebar) View() string {
	// Pre-style the Online title based on focus.
	titleC := ColorFgDim
	if s.outerFocused && s.focused == PaneBooted {
		titleC = ColorBorderHi
	}
	onlineTitle := lipgloss.NewStyle().Foreground(titleC).Background(ColorBg).Render("Online")

	onlineBox := renderBoxRaw(
		onlineTitle,
		s.renderOnlineRows(),
		"",
		s.width, 0,
		s.outerFocused && s.focused == PaneBooted,
		"1",
		s.onlinePaginationStr(),
	)

	offlineHeight := 0
	if s.height > 0 {
		const gapLines = 1
		offlineHeight = max(s.height-lipgloss.Height(onlineBox)-gapLines, 3)
	}

	var offlineFooter string
	if s.filterMode || s.filterQuery != "" {
		offlineFooter = s.renderFilterRow(s.width - 2)
	}

	offlineBox := renderBoxRaw(
		s.renderOfflineTitle(),
		s.renderOfflineRows(),
		offlineFooter,
		s.width, offlineHeight,
		s.outerFocused && s.focused == PaneAvailable,
		"2",
		s.offlinePaginationStr(),
	)

	gap := lipgloss.NewStyle().Background(ColorBg).Width(s.width).Render("")
	return lipgloss.JoinVertical(lipgloss.Left, onlineBox, gap, offlineBox)
}

// ── Online box internals ─────────────────────────────────────────────────────

// onlinePos is one cursor stop in the Online pane.
type onlinePos struct {
	isHeader  bool
	platform  device.Platform
	bootedIdx int // index into s.booted (valid when !isHeader)
}

// onlinePositions returns the ordered cursor stops for the Online box,
// accounting for collapsed groups. Returns nil when booted is empty.
func (s Sidebar) onlinePositions() []onlinePos {
	var iosIdx, andIdx []int
	for i, d := range s.booted {
		if d.Platform == device.PlatformIOS {
			iosIdx = append(iosIdx, i)
		} else {
			andIdx = append(andIdx, i)
		}
	}

	var out []onlinePos
	if len(iosIdx) > 0 {
		out = append(out, onlinePos{isHeader: true, platform: device.PlatformIOS})
		if !s.iosCollapsed {
			for _, i := range iosIdx {
				out = append(out, onlinePos{platform: device.PlatformIOS, bootedIdx: i})
			}
		}
	}
	if len(andIdx) > 0 {
		out = append(out, onlinePos{isHeader: true, platform: device.PlatformAndroid})
		if !s.androidCollapsed {
			for _, i := range andIdx {
				out = append(out, onlinePos{platform: device.PlatformAndroid, bootedIdx: i})
			}
		}
	}
	return out
}

// onlineDeviceAt returns the device at the given Online cursor index,
// or nil if the position is a header or out of range.
func (s Sidebar) onlineDeviceAt(idx int) *device.Device {
	positions := s.onlinePositions()
	if idx < 0 || idx >= len(positions) {
		return nil
	}
	p := positions[idx]
	if p.isHeader {
		return nil
	}
	return &s.booted[p.bootedIdx]
}

// toggleOnlineGroup flips the collapsed state of the group whose header the
// cursor is on. No-op if cursor is on a device row or positions is empty.
func (s *Sidebar) toggleOnlineGroup() {
	positions := s.onlinePositions()
	if s.bootedIdx < 0 || s.bootedIdx >= len(positions) {
		return
	}
	cur := positions[s.bootedIdx]
	if !cur.isHeader {
		return
	}
	switch cur.platform {
	case device.PlatformIOS:
		s.iosCollapsed = !s.iosCollapsed
	case device.PlatformAndroid:
		s.androidCollapsed = !s.androidCollapsed
	}
	for i, p := range s.onlinePositions() {
		if p.isHeader && p.platform == cur.platform {
			s.bootedIdx = i
			return
		}
	}
}

// onlinePaginationStr returns "X of N" for the Online box bottom border,
// where N = total booted devices and X = sequential ordinal at the cursor.
func (s Sidebar) onlinePaginationStr() string {
	if len(s.booted) == 0 {
		return ""
	}
	n := len(s.booted)
	positions := s.onlinePositions()
	if len(positions) == 0 || s.bootedIdx >= len(positions) {
		return fmt.Sprintf("1 of %d", n)
	}

	iosCount := 0
	for _, d := range s.booted {
		if d.Platform == device.PlatformIOS {
			iosCount++
		}
	}

	p := positions[s.bootedIdx]
	var x int
	if p.isHeader {
		if p.platform == device.PlatformIOS {
			x = 1
		} else {
			x = iosCount + 1
		}
	} else {
		x = s.deviceOrdinal(p.bootedIdx)
	}

	return fmt.Sprintf("%d of %d", x, n)
}

// deviceOrdinal returns the 1-based ordinal of booted[idx] in iOS-first order.
func (s Sidebar) deviceOrdinal(idx int) int {
	iosCount := 0
	for _, d := range s.booted {
		if d.Platform == device.PlatformIOS {
			iosCount++
		}
	}
	d := s.booted[idx]
	if d.Platform == device.PlatformIOS {
		cnt := 0
		for i, b := range s.booted {
			if b.Platform == device.PlatformIOS {
				cnt++
				if i == idx {
					return cnt
				}
			}
		}
	} else {
		cnt := 0
		for i, b := range s.booted {
			if b.Platform == device.PlatformAndroid {
				cnt++
				if i == idx {
					return iosCount + cnt
				}
			}
		}
	}
	return 1
}

// renderOnlineRows builds the content string for the Online box.
func (s Sidebar) renderOnlineRows() string {
	innerW := s.width - 2
	positions := s.onlinePositions()
	if len(positions) == 0 {
		return renderEmptyRow("(none)", innerW)
	}

	iosCount, andCount := 0, 0
	for _, d := range s.booted {
		if d.Platform == device.PlatformIOS {
			iosCount++
		} else {
			andCount++
		}
	}

	var lines []string
	prevPlatform := device.Platform("")
	for i, p := range positions {
		sel := s.focused == PaneBooted && i == s.bootedIdx
		if p.isHeader {
			if prevPlatform != "" && prevPlatform != p.platform {
				lines = append(lines, renderEmptyRow("", innerW))
			}
			var glyph, label string
			var clr color.Color
			var count int
			var collapsed bool
			if p.platform == device.PlatformIOS {
				glyph, label, count, collapsed = "⌘", "iOS", iosCount, s.iosCollapsed
				clr = ColorIOS
			} else {
				glyph, label, count, collapsed = "⛯", "Android", andCount, s.androidCollapsed
				clr = ColorAndroid
			}
			lines = append(lines, renderGroupHeader(glyph, label, count, clr, innerW, collapsed, sel))
			prevPlatform = p.platform
		} else {
			lines = append(lines, renderDeviceRow(s.booted[p.bootedIdx], sel, innerW, true))
		}
	}
	return strings.Join(lines, "\n")
}

// ── Offline box internals ────────────────────────────────────────────────────

// offlineDevices returns the filtered device list for the current offline tab.
func (s Sidebar) offlineDevices() []device.Device {
	if s.offlinePlatform == tabIOS {
		return s.filteredVirtIOS()
	}
	return s.filteredVirtAnd()
}

// offlinePaginationStr returns "X of N" for the Offline box bottom border.
func (s Sidebar) offlinePaginationStr() string {
	devs := s.offlineDevices()
	if len(devs) == 0 {
		return ""
	}
	return fmt.Sprintf("%d of %d", s.availIdx+1, len(devs))
}

// renderOfflineTitle returns the pre-styled "iOS ─ Android" tab title.
func (s Sidebar) renderOfflineTitle() string {
	isFocused := s.outerFocused && s.focused == PaneAvailable
	sep := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(" ─ ")

	var iosStyle, andStyle lipgloss.Style
	if isFocused {
		active := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
		inactive := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
		if s.offlinePlatform == tabIOS {
			iosStyle, andStyle = active, inactive
		} else {
			iosStyle, andStyle = inactive, active
		}
	} else {
		dim := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
		iosStyle, andStyle = dim, dim
	}

	return iosStyle.Render("iOS") + sep + andStyle.Render("Android")
}

// renderOfflineRows builds the content string for the Offline box.
func (s Sidebar) renderOfflineRows() string {
	innerW := s.width - 2
	devs := s.offlineDevices()

	var allVirt []device.Device
	if s.offlinePlatform == tabIOS {
		allVirt = s.virtIOS
	} else {
		allVirt = s.virtAnd
	}

	if len(allVirt) == 0 {
		return renderEmptyRow("(none)", innerW)
	}
	if len(devs) == 0 {
		return renderEmptyRow("no match", innerW)
	}

	lines := make([]string, 0, len(devs))
	for i, d := range devs {
		sel := s.focused == PaneAvailable && i == s.availIdx
		lines = append(lines, renderDeviceRow(d, sel, innerW, false))
	}
	return strings.Join(lines, "\n")
}

// ── Shared helpers ───────────────────────────────────────────────────────────

func (s *Sidebar) moveCursor(d int) {
	switch s.focused {
	case PaneBooted:
		if n := len(s.onlinePositions()); n > 0 {
			s.bootedIdx = clampIdx(s.bootedIdx+d, n)
		}
	case PaneAvailable:
		if n := len(s.offlineDevices()); n > 0 {
			s.availIdx = clampIdx(s.availIdx+d, n)
		}
	}
}

func (s Sidebar) filteredVirtIOS() []device.Device {
	if s.filterQuery == "" {
		return s.virtIOS
	}
	out := make([]device.Device, 0, len(s.virtIOS))
	for _, d := range s.virtIOS {
		if fuzzyMatch(s.filterQuery, d.Name) {
			out = append(out, d)
		}
	}
	return out
}

func (s Sidebar) filteredVirtAnd() []device.Device {
	if s.filterQuery == "" {
		return s.virtAnd
	}
	out := make([]device.Device, 0, len(s.virtAnd))
	for _, d := range s.virtAnd {
		if fuzzyMatch(s.filterQuery, d.Name) {
			out = append(out, d)
		}
	}
	return out
}

func (s Sidebar) renderFilterRow(innerW int) string {
	cursor := ""
	if s.filterMode {
		cursor = "█"
	}
	query := s.filterQuery + cursor
	prefix := " / "
	row := prefix + query
	w := lipgloss.Width(row)
	queryStyle := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	bg := lipgloss.NewStyle().Background(ColorBg)
	rendered := queryStyle.Render(row)
	if w < innerW {
		return rendered + bg.Render(strings.Repeat(" ", innerW-w))
	}
	return rendered
}

// fuzzyMatch reports whether all runes of query appear in target in order,
// case-insensitive. An empty query matches everything.
func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}
	target = strings.ToLower(target)
	query = strings.ToLower(query)
	qi := 0
	for _, ch := range target {
		if qi < len([]rune(query)) && ch == []rune(query)[qi] {
			qi++
		}
	}
	return qi == len([]rune(query))
}

// rowInnerPad is the cell padding applied to both the left and right inner
// edges of every sidebar row. groupIndent is added to the left for rows nested
// under a group header. minNameVerGap is the minimum number of cells reserved
// between the device name and the runtime version.
const (
	rowInnerPad   = 1
	groupIndent   = 2
	minNameVerGap = 2
)

// truncateName clips name to at most max visible cells, appending "…" if the
// name was shortened. Returns "" if max <= 0.
func truncateName(name string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(name) <= max {
		return name
	}
	if max == 1 {
		return "…"
	}
	runes := []rune(name)
	for i := len(runes) - 1; i > 0; i-- {
		if lipgloss.Width(string(runes[:i])) <= max-1 {
			return string(runes[:i]) + "…"
		}
	}
	return "…"
}

// renderDeviceRow returns a single line of visible width == innerW.
func renderDeviceRow(dev device.Device, selected bool, innerW int, indent bool) string {
	leftPad := strings.Repeat(" ", rowInnerPad)
	if indent {
		leftPad = strings.Repeat(" ", rowInnerPad+groupIndent)
	}
	rightPad := strings.Repeat(" ", rowInnerPad)

	dotC := ColorFgFaint
	if dev.Status == device.StatusRunning {
		dotC = ColorOk
	}

	ver := dev.Version
	verW := lipgloss.Width(ver)

	// selected: full-row highlight, dot + name
	if selected {
		// prefix: leftPad + dot(1) + space(1)
		prefixW := len(leftPad) + 2
		nameMax := innerW - prefixW - minNameVerGap - verW - len(rightPad)
		nameStr := truncateName(dev.Name, nameMax)
		gap := max(innerW-prefixW-lipgloss.Width(nameStr)-verW-len(rightPad), minNameVerGap)
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		dotSel := lipgloss.NewStyle().Foreground(dotC).Background(ColorAccent).Bold(true).Render("●")
		return sel.Render(leftPad) + dotSel + sel.Render(" "+nameStr+strings.Repeat(" ", gap)+ver+rightPad)
	}

	// non-selected: leftPad + dot + space + name + gap + version + rightPad
	prefixW := len(leftPad) + 2 // dot + space
	nameMax := innerW - prefixW - minNameVerGap - verW - len(rightPad)
	nameStr := truncateName(dev.Name, nameMax)
	gap := max(innerW-prefixW-lipgloss.Width(nameStr)-verW-len(rightPad), minNameVerGap)

	dot := lipgloss.NewStyle().Foreground(dotC).Background(ColorBg).Render("●")
	nameC := ColorFg
	if dev.Status != device.StatusRunning {
		nameC = ColorFgDim
	}
	name := lipgloss.NewStyle().Foreground(nameC).Background(ColorBg).Render(nameStr)
	meta := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(ver)
	bg := lipgloss.NewStyle().Background(ColorBg)

	return leftPad + dot + " " + name + bg.Render(strings.Repeat(" ", gap)) + meta + bg.Render(rightPad)
}

func renderGroupHeader(glyph, label string, count int, c color.Color, innerW int, collapsed, selected bool) string {
	caretCh := "▾"
	if collapsed {
		caretCh = "▸"
	}
	labelText := label
	if glyph != "" {
		labelText = glyph + " " + label
	}

	leftPad := strings.Repeat(" ", rowInnerPad)
	rightPad := strings.Repeat(" ", rowInnerPad)

	if selected {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		text := leftPad + caretCh + " " + labelText + " " + fmt.Sprintf("(%d)", count)
		gap := max(innerW-lipgloss.Width(text)-len(rightPad), 1)
		return sel.Render(text + strings.Repeat(" ", gap) + rightPad)
	}

	bg := lipgloss.NewStyle().Background(ColorBg)
	caret := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(caretCh)
	nameStyle := lipgloss.NewStyle().Foreground(c).Background(ColorBg).Bold(true).Render(labelText)
	cnt := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(fmt.Sprintf("(%d)", count))
	row := leftPad + caret + " " + nameStyle + " " + cnt
	w := lipgloss.Width(row)
	if w >= innerW {
		return row
	}
	return row + bg.Render(strings.Repeat(" ", innerW-w-len(rightPad))) + bg.Render(rightPad)
}

func renderEmptyRow(text string, innerW int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)
	if text == "" {
		return bg.Render(strings.Repeat(" ", innerW))
	}
	body := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render("  " + text)
	w := lipgloss.Width(body)
	if w >= innerW {
		return body
	}
	return body + bg.Render(strings.Repeat(" ", innerW-w))
}

func clampIdx(i, n int) int {
	if n <= 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
