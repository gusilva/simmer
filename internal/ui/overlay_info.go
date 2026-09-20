package ui

import (
	"os/exec"
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// InfoOverlay is the dismissible right-side sheet showing device metadata. It is
// modal-ish: while open it owns the keyboard (esc close, c copy udid, r reveal),
// and nothing beneath it reflows.
type InfoOverlay struct {
	dev    device.Device
	info   device.DeviceInfo
	width  int
	height int
}

// NewInfoOverlay builds the sheet for a device. w/h are the full terminal size.
func NewInfoOverlay(dev device.Device, info device.DeviceInfo, w, h int) InfoOverlay {
	return InfoOverlay{dev: dev, info: info, width: w, height: h}
}

// RevealedMsg is dispatched after RevealInFinderCmd runs.
type RevealedMsg struct {
	Path string
	Err  error
}

// RevealInFinderCmd opens a Finder window with the given path selected.
func RevealInFinderCmd(path string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("open", "-R", path).Run()
		return RevealedMsg{Path: path, Err: err}
	}
}

// revealPath returns a host filesystem path to reveal in Finder, or "" when the
// device has none (e.g. physical devices).
func (o InfoOverlay) revealPath() string {
	for _, f := range o.info.Fields {
		if f.Key == "Data Path" && strings.HasPrefix(f.Value, "/") {
			return f.Value
		}
	}
	return ""
}

// Update handles the sheet's own keys. All other keys are swallowed so the tree
// beneath stays frozen.
func (o InfoOverlay) Update(msg tea.Msg) (InfoOverlay, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	switch k.String() {
	case "esc":
		return o, func() tea.Msg { return CancelOverlayMsg{} }
	case "c":
		return o, CopyToClipboardCmd(o.dev.ID)
	case "r":
		if p := o.revealPath(); p != "" {
			return o, RevealInFinderCmd(p)
		}
	}
	return o, nil
}

// View renders the sheet as a full-height bordered box for compositor
// placement against the right edge.
func (o InfoOverlay) View() string {
	sheetW := min(max(o.width*40/100, 34), o.width)
	const padX = 2
	innerW := max(sheetW-2-padX*2, 12)

	boxH := max(o.height, 8)
	innerH := max(boxH-2-2, 4) // border (2) + vertical padding (1+1)

	bg := lipgloss.NewStyle().Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	title := titleStyle.Render("Device info")
	closeHint := faint.Render("[esc] close")
	hgap := max(innerW-lipgloss.Width(title)-lipgloss.Width(closeHint), 1)
	header := " " + title + bg.Render(strings.Repeat(" ", hgap)) + closeHint

	lines := []string{header, ""}
	if len(o.info.Fields) == 0 {
		lines = append(lines, faint.Render("  loading info…"))
	} else {
		keyW := infoKeyWidth(o.info.Fields)
		for _, f := range o.info.Fields {
			lines = append(lines, renderInfoRow(f.Key, f.Value, keyW, innerW, false))
		}
	}

	footer := " " + faint.Render("c copy udid")
	if o.revealPath() != "" {
		footer += faint.Render("   r reveal in Finder")
	}

	// Reserve the last inner row for the footer; pad/clip the body to fit.
	for len(lines) < innerH-1 {
		lines = append(lines, "")
	}
	if len(lines) > innerH-1 {
		lines = lines[:innerH-1]
	}
	lines = append(lines, footer)

	for i, ln := range lines {
		if pad := innerW - lipgloss.Width(ln); pad > 0 {
			lines[i] = ln + bg.Render(strings.Repeat(" ", pad))
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, padX)
	return box.Render(strings.Join(lines, "\n"))
}

// infoKeyWidth picks the column width for keys: longest key, capped at 14 so a
// single long label doesn't squeeze the value column.
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
