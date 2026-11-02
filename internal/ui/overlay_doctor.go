package ui

import (
	"image/color"
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// RunDoctorMsg asks the app to (re)run the Doctor report.
type RunDoctorMsg struct{}

// DoctorOverlay is the right-side sheet showing the Doctor report: the same
// health checks `simmer doctor` prints. It reports and prescribes; starting
// the adb server is a separate key, never a side effect of opening it.
type DoctorOverlay struct {
	checks  []device.HealthCheck
	loading bool
	width   int
	height  int
}

// NewDoctorOverlay builds the sheet in its loading state. w/h are the full
// terminal size.
func NewDoctorOverlay(w, h int) DoctorOverlay {
	return DoctorOverlay{loading: true, width: w, height: h}
}

// SetChecks fills the sheet with a finished report.
func (o *DoctorOverlay) SetChecks(checks []device.HealthCheck) {
	o.checks = checks
	o.loading = false
}

// Update handles the sheet's own keys: esc closes, r re-runs. Everything
// else is swallowed so nothing beneath reacts.
func (o DoctorOverlay) Update(msg tea.Msg) (DoctorOverlay, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	switch k.String() {
	case "esc":
		return o, func() tea.Msg { return CancelOverlayMsg{} }
	case "r":
		if !o.loading {
			o.loading = true
			return o, func() tea.Msg { return RunDoctorMsg{} }
		}
	}
	return o, nil
}

// View renders the sheet as a full-height bordered box for compositor
// placement against the right edge.
func (o DoctorOverlay) View() string {
	sheetW := min(max(o.width*60/100, 52), o.width)
	const padX = 2
	innerW := max(sheetW-2-padX*2, 16)

	boxH := max(o.height, 8)
	innerH := max(boxH-2-2, 4)

	bg := lipgloss.NewStyle().Background(ColorBg)
	titleStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	title := titleStyle.Render("Doctor")
	closeHint := faint.Render("[esc] close")
	hgap := max(innerW-lipgloss.Width(title)-lipgloss.Width(closeHint), 1)
	lines := []string{" " + title + bg.Render(strings.Repeat(" ", hgap)) + closeHint, ""}

	if o.loading {
		lines = append(lines, faint.Render("  checking…"))
	} else {
		lines = append(lines, doctorLines(o.checks, innerW)...)
	}

	footer := " " + faint.Render("r re-check")
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

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, padX).
		Render(strings.Join(lines, "\n"))
}

// doctorLines groups checks under their section and adds a fix line under
// every check that has one.
func doctorLines(checks []device.HealthCheck, w int) []string {
	section := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Bold(true)
	name := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	detail := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	fix := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg)

	const nameW = 20
	var out []string
	prev := ""
	for _, c := range checks {
		if c.Section != prev {
			if prev != "" {
				out = append(out, "")
			}
			out = append(out, " "+section.Render(strings.ToUpper(c.Section)))
			prev = c.Section
		}
		mark, clr := severityGlyph(c.Severity)
		n := padRight(truncateName(c.Name, nameW), nameW)
		lead := " " + lipgloss.NewStyle().Foreground(clr).Background(ColorBg).Render(mark) + " " + name.Render(n) + " "
		// Details and fixes are the point of the sheet, so they wrap under
		// the detail column rather than being cut off.
		leadW := lipgloss.Width(lead)
		hang := strings.Repeat(" ", leadW)
		for i, ln := range strings.Split(ansi.Wordwrap(c.Detail, max(w-leadW, 8), " /·"), "\n") {
			if i == 0 {
				out = append(out, lead+detail.Render(ln))
			} else {
				out = append(out, hang+detail.Render(ln))
			}
		}
		if c.Fix != "" {
			for _, ln := range strings.Split(ansi.Wordwrap("→ "+c.Fix, max(w-leadW, 8), " /"), "\n") {
				out = append(out, hang+fix.Render(ln))
			}
		}
	}
	return out
}

func severityGlyph(s device.Severity) (string, color.Color) {
	switch s {
	case device.SeverityFail:
		return "✗", ColorErr
	case device.SeverityWarn:
		return "!", ColorWarn
	default:
		return "✓", ColorOk
	}
}
