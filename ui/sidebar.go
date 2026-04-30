package ui

import (
	"fmt"
	"image/color"
	"strings"

	"simmer/pkg/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// SidebarPane identifies which sub-panel of the sidebar holds focus.
type SidebarPane int

const (
	// PaneBooted is the top "Booted" panel.
	PaneBooted SidebarPane = iota
	// PaneAvailable is the bottom "Available" panel.
	PaneAvailable
)

// DefaultSidebarWidth is the column width used when none is set.
const DefaultSidebarWidth = 36

// Sidebar is the left navigation panel. It is a pure UI component:
// callers feed it a flat []device.Device via SetDevices and the sidebar
// handles grouping, selection, and rendering.
type Sidebar struct {
	booted   []device.Device
	iosAvail []device.Device
	andAvail []device.Device

	focused      SidebarPane
	outerFocused bool
	bootedIdx    int
	availIdx     int

	iosCollapsed     bool
	androidCollapsed bool

	width  int
	height int
}

// NewSidebar returns a sidebar with focus on the booted panel.
func NewSidebar() Sidebar {
	return Sidebar{focused: PaneBooted, outerFocused: true, width: DefaultSidebarWidth}
}

// SetFocused marks whether the sidebar holds the app's outer focus. When
// unfocused, both panels render with the dim border color and inner pane
// highlights are suppressed.
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

// SetDevices replaces the device list, splitting devices into booted and
// available groups by platform. Selection indices are clamped to remain valid.
func (s *Sidebar) SetDevices(devs []device.Device) {
	s.booted = s.booted[:0]
	s.iosAvail = s.iosAvail[:0]
	s.andAvail = s.andAvail[:0]
	for _, d := range devs {
		if d.Status == device.StatusRunning {
			s.booted = append(s.booted, d)
			continue
		}
		switch d.Platform {
		case device.PlatformIOS:
			s.iosAvail = append(s.iosAvail, d)
		case device.PlatformAndroid:
			s.andAvail = append(s.andAvail, d)
		}
	}
	s.bootedIdx = clampIdx(s.bootedIdx, len(s.booted))
	s.availIdx = clampIdx(s.availIdx, s.availableLen())
}

// SelectedDevice returns the device under the cursor in the focused pane,
// or nil if the focused pane is empty.
// Devices returns all devices currently loaded in the sidebar (booted + available).
func (s Sidebar) Devices() []device.Device {
	out := make([]device.Device, 0, len(s.booted)+len(s.iosAvail)+len(s.andAvail))
	out = append(out, s.booted...)
	out = append(out, s.iosAvail...)
	out = append(out, s.andAvail...)
	return out
}

func (s Sidebar) SelectedDevice() *device.Device {
	switch s.focused {
	case PaneBooted:
		if s.bootedIdx < len(s.booted) {
			return &s.booted[s.bootedIdx]
		}
	case PaneAvailable:
		return s.availDeviceAt(s.availIdx)
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
	switch k.String() {
	case "up", "k":
		s.moveCursor(-1)
	case "down", "j":
		s.moveCursor(1)
	case "tab", "right", "l", "left", "h", "shift+tab":
		if s.focused == PaneBooted {
			s.focused = PaneAvailable
		} else {
			s.focused = PaneBooted
		}
	case "enter":
		if s.focused == PaneAvailable {
			s.toggleCurrentGroup()
		}
	}
	return s, nil
}

// View renders the sidebar as a multi-line string. The Booted panel is sized
// to fit its content; the Available panel grows to fill the remaining height
// when SetSize was called with a positive height.
func (s Sidebar) View() string {
	bootedBox := RenderBox(
		"Booted",
		fmt.Sprintf("%d", len(s.booted)),
		s.renderBootedRows(),
		s.width,
		0,
		s.outerFocused && s.focused == PaneBooted,
	)

	availHeight := 0
	if s.height > 0 {
		const gapLines = 1
		availHeight = max(s.height-lipgloss.Height(bootedBox)-gapLines, 3)
	}

	availBox := RenderBox(
		"Available",
		fmt.Sprintf("%d", len(s.iosAvail)+len(s.andAvail)),
		s.renderAvailableRows(),
		s.width,
		availHeight,
		s.outerFocused && s.focused == PaneAvailable,
	)
	gap := lipgloss.NewStyle().Background(ColorBg).Width(s.width).Render("")
	return lipgloss.JoinVertical(lipgloss.Left, bootedBox, gap, availBox)
}

// ── internals ────────────────────────────────────────────────────────────

func (s *Sidebar) moveCursor(d int) {
	switch s.focused {
	case PaneBooted:
		if n := len(s.booted); n > 0 {
			s.bootedIdx = clampIdx(s.bootedIdx+d, n)
		}
	case PaneAvailable:
		if n := s.availableLen(); n > 0 {
			s.availIdx = clampIdx(s.availIdx+d, n)
		}
	}
}

// availPos identifies one cursor stop in the Available pane: either a
// group header or a device row. Group: 0 = iOS, 1 = Android.
type availPos struct {
	isHeader bool
	group    int
	devIdx   int
}

// availPositions returns the ordered list of cursor stops in the Available
// pane, accounting for collapsed groups. Each group with at least one device
// contributes a header position; expanded groups also contribute one position
// per device.
func (s Sidebar) availPositions() []availPos {
	var out []availPos
	if len(s.iosAvail) > 0 {
		out = append(out, availPos{isHeader: true, group: 0})
		if !s.iosCollapsed {
			for i := range s.iosAvail {
				out = append(out, availPos{group: 0, devIdx: i})
			}
		}
	}
	if len(s.andAvail) > 0 {
		out = append(out, availPos{isHeader: true, group: 1})
		if !s.androidCollapsed {
			for i := range s.andAvail {
				out = append(out, availPos{group: 1, devIdx: i})
			}
		}
	}
	return out
}

func (s Sidebar) availableLen() int { return len(s.availPositions()) }

func (s Sidebar) availDeviceAt(idx int) *device.Device {
	positions := s.availPositions()
	if idx < 0 || idx >= len(positions) {
		return nil
	}
	p := positions[idx]
	if p.isHeader {
		return nil
	}
	if p.group == 0 {
		return &s.iosAvail[p.devIdx]
	}
	return &s.andAvail[p.devIdx]
}

// toggleCurrentGroup flips the collapsed state of the group whose header the
// cursor is on. No-op if the cursor is on a device row or no positions exist.
// After toggling, the cursor is re-anchored onto the same header.
func (s *Sidebar) toggleCurrentGroup() {
	positions := s.availPositions()
	if s.availIdx < 0 || s.availIdx >= len(positions) {
		return
	}
	cur := positions[s.availIdx]
	if !cur.isHeader {
		return
	}
	switch cur.group {
	case 0:
		s.iosCollapsed = !s.iosCollapsed
	case 1:
		s.androidCollapsed = !s.androidCollapsed
	}
	for i, p := range s.availPositions() {
		if p.isHeader && p.group == cur.group {
			s.availIdx = i
			return
		}
	}
}

func (s Sidebar) renderBootedRows() string {
	innerW := s.width - 2
	if len(s.booted) == 0 {
		return renderEmptyRow("(none)", innerW)
	}
	lines := make([]string, 0, len(s.booted))
	for i, d := range s.booted {
		sel := s.focused == PaneBooted && i == s.bootedIdx
		lines = append(lines, renderDeviceRow(d, sel, innerW, false))
	}
	return strings.Join(lines, "\n")
}

func (s Sidebar) renderAvailableRows() string {
	innerW := s.width - 2
	if len(s.iosAvail) == 0 && len(s.andAvail) == 0 {
		return renderEmptyRow("(none)", innerW)
	}

	positions := s.availPositions()
	var lines []string
	prevGroup := -1
	for i, p := range positions {
		sel := s.focused == PaneAvailable && i == s.availIdx
		if p.isHeader {
			if prevGroup != -1 && prevGroup != p.group {
				lines = append(lines, renderEmptyRow("", innerW))
			}
			glyph, label, color, count, collapsed := groupHeaderArgs(s, p.group)
			lines = append(lines, renderGroupHeader(glyph, label, count, color, innerW, collapsed, sel))
			prevGroup = p.group
			continue
		}
		var dev device.Device
		if p.group == 0 {
			dev = s.iosAvail[p.devIdx]
		} else {
			dev = s.andAvail[p.devIdx]
		}
		lines = append(lines, renderDeviceRow(dev, sel, innerW, true))
	}
	return strings.Join(lines, "\n")
}

func groupHeaderArgs(s Sidebar, group int) (glyph, label string, c color.Color, count int, collapsed bool) {
	if group == 0 {
		return "", "iOS", ColorIOS, len(s.iosAvail), s.iosCollapsed
	}
	return "▲", "Android", ColorAndroid, len(s.andAvail), s.androidCollapsed
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

// renderDeviceRow returns a single line of visible width == innerW. The device
// name hugs the left edge (after pad/indent) and the runtime version hugs the
// right edge — like CSS flex justify-between. Long names are truncated with
// an ellipsis so at least minNameVerGap cells separate name from version.
func renderDeviceRow(dev device.Device, selected bool, innerW int, indent bool) string {
	leftPad := strings.Repeat(" ", rowInnerPad)
	if indent {
		leftPad = strings.Repeat(" ", rowInnerPad+groupIndent)
	}
	rightPad := strings.Repeat(" ", rowInnerPad)

	pglyph := ""
	pgC := ColorIOS
	if dev.Platform == device.PlatformAndroid {
		pglyph = "▲"
		pgC = ColorAndroid
	}

	dotC := ColorFgFaint
	if dev.Status == device.StatusRunning {
		dotC = ColorOk
	}

	ver := dev.Version
	verW := lipgloss.Width(ver)

	if selected {
		// prefix: leftPad + pglyph + " "
		prefixW := len(leftPad) + lipgloss.Width(pglyph) + 1
		nameMax := innerW - prefixW - minNameVerGap - verW - len(rightPad)
		nameStr := truncateName(dev.Name, nameMax)
		gap := max(innerW-prefixW-lipgloss.Width(nameStr)-verW-len(rightPad), minNameVerGap)
		bg := lipgloss.NewStyle().
			Foreground(ColorBg).
			Background(ColorAccent).
			Bold(true)
		return bg.Render(leftPad + pglyph + " " + nameStr + strings.Repeat(" ", gap) + ver + rightPad)
	}

	// non-selected prefix: leftPad + dot(1) + " "(1) + glyph(1) + " "(1)
	prefixW := len(leftPad) + 1 + 1 + lipgloss.Width(pglyph) + 1
	nameMax := innerW - prefixW - minNameVerGap - verW - len(rightPad)
	nameStr := truncateName(dev.Name, nameMax)
	gap := max(innerW-prefixW-lipgloss.Width(nameStr)-verW-len(rightPad), minNameVerGap)

	dot := lipgloss.NewStyle().Foreground(dotC).Background(ColorBg).Render("●")
	glyph := lipgloss.NewStyle().Foreground(pgC).Background(ColorBg).Render(pglyph)
	nameC := ColorFg
	if dev.Status != device.StatusRunning {
		nameC = ColorFgDim
	}
	name := lipgloss.NewStyle().Foreground(nameC).Background(ColorBg).Render(nameStr)
	meta := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(ver)
	bg := lipgloss.NewStyle().Background(ColorBg)

	return leftPad + dot + " " + glyph + " " + name + bg.Render(strings.Repeat(" ", gap)) + meta + bg.Render(rightPad)
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
	name := lipgloss.NewStyle().Foreground(c).Background(ColorBg).Bold(true).Render(labelText)
	cnt := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(fmt.Sprintf("(%d)", count))
	row := leftPad + caret + " " + name + " " + cnt
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
