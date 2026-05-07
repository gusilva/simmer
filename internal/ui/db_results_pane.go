package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// DBResultsPane renders the query results below the horizontal divider.
// Navigation is handled by the Bubbles v2 table component.
type DBResultsPane struct {
	tbl table.Model
}

// fixedRenderedW is the sum of all column rendered widths (content + padding 0,1 = +2)
// except the flex "name" column:
//
//	#(3+2) + udid(14+2) + os(8+2) + state(10+2) + runtime(7+2) +
//	apps(4+2) + cpu_pct(7+2) + booted_at(16+2) = 85
const fixedRenderedW = 85

func resultsColumns(nameW int) []table.Column {
	return []table.Column{
		{Title: "#", Width: 3},
		{Title: "⚿ udid", Width: 14},
		{Title: "name", Width: nameW},
		{Title: "os", Width: 8},
		{Title: "state", Width: 10},
		{Title: "runtime", Width: 7},
		{Title: "apps", Width: 4},
		{Title: "cpu_pct", Width: 7},
		{Title: "booted_at", Width: 16},
	}
}

func resultsRows() []table.Row {
	return []table.Row{
		{"1", "9C3A2F-…-71BE", "iPhone 17 Pro", "iOS", "● Booted", "26.1", "142", "12.4", "2026-04-28 13:40"},
		{"2", "F2E4A1-…-A09D", "iPhone 17", "iOS", "● Booted", "26.1", "87", "4.1", "2026-04-28 11:08"},
		{"3", "B71D08-…-3F12", "iPhone 16 Pro Max", "iOS", "◐ Booting", "26.0", "54", "22.7", "2026-04-28 10:44"},
		{"4", "3A0928-…-CE51", `iPad Pro 13"`, "iPadOS", "○ Shutdown", "26.1", "0", "NULL", "2026-04-27 18:22"},
		{"5", "5D33C9-…-9182", "Pixel 9 Pro", "Android", "● Booted", "15.0", "33", "8.6", "2026-04-28 09:11"},
		{"6", "E81F44-…-7AB2", "Pixel 9", "Android", "○ Shutdown", "15.0", "0", "NULL", "2026-04-26 22:06"},
		{"7", "2F87C3-…-D040", "Galaxy S25 Ultra", "Android", "● Booted", "15.0", "61", "7.9", "2026-04-28 08:49"},
		{"8", "7B9211-…-15CA", "Apple Watch S10", "watchOS", "◐ Booting", "12.1", "4", "15.2", "2026-04-28 07:33"},
		{"9", "CC4E1A-…-8E07", "Apple TV 4K", "tvOS", "○ Shutdown", "19.0", "2", "NULL", "2026-04-25 16:58"},
		{"10", "81A655-…-B339", "Vision Pro", "visionOS", "● Booted", "3.2", "11", "18.0", "2026-04-28 06:14"},
		{"11", "A0DE77-…-F284", "iPhone 15 Pro", "iOS", "○ Shutdown", "26.1", "0", "NULL", "2026-04-24 19:22"},
		{"12", "D5B3F0-…-C711", "Pixel Tablet", "Android", "○ Shutdown", "15.0", "0", "NULL", "2026-04-23 12:01"},
		{"13", "4F2B89-…-A0DE", "Galaxy Z Fold 6", "Android", "○ Shutdown", "14.0", "0", "NULL", "2026-04-22 09:45"},
		{"14", "61E3C8-…-7DD4", "iPhone SE (3rd gen)", "iOS", "○ Shutdown", "26.1", "0", "NULL", "2026-04-21 14:07"},
	}
}

// sentinelBg is an otherwise-unused colour used to tag the selected row in
// table.View() output so we can identify and re-render it at full width.
const sentinelBg = "#000102"

func resultsStyles() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().
			Background(ColorTableHeader).
			Foreground(ColorFg).
			Bold(true).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Foreground(ColorFgDim).
			Padding(0, 1),
		// The real highlight is applied in Rows() after stripping inner ANSI
		// resets that would cancel this background. sentinelBg lets us find the
		// selected line in the rendered output.
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(sentinelBg)),
	}
}

func newDBResultsPane() DBResultsPane {
	t := table.New(
		table.WithColumns(resultsColumns(20)),
		table.WithRows(resultsRows()),
		table.WithStyles(resultsStyles()),
		table.WithFocused(false),
	)
	return DBResultsPane{tbl: t}
}

// setHeight tells the table how many rows it can occupy (header + data).
// Called from DBViewerModal.SetSize.
func (p *DBResultsPane) setHeight(h int) {
	// Layout overhead: filterRow(1) + filterSep(1) + pagerSep(1) + pager(1) = 4
	p.tbl.SetHeight(max(h-4, 3))
}

func (p DBResultsPane) Update(msg tea.Msg) (DBResultsPane, tea.Cmd) {
	// Ensure the table handles key events.
	p.tbl.Focus()
	var cmd tea.Cmd
	p.tbl, cmd = p.tbl.Update(msg)
	return p, cmd
}

// Rows returns height strings each exactly width visual cells wide.
//
// Layout:
//
//	row 0        WHERE filter bar
//	row 1        filter separator ──────
//	rows 2..h-3  table.View() — header + scrollable data rows
//	row h-2      pager separator ──────
//	row h-1      pager
func (p DBResultsPane) Rows(width, height int, focused bool) []string {
	bgS := lipgloss.NewStyle().Background(ColorBg)
	blankN := func(n int) string {
		if n <= 0 {
			return ""
		}
		return bgS.Render(strings.Repeat(" ", n))
	}
	fillTo := func(s string) string {
		need := width - lipgloss.Width(s)
		if need <= 0 {
			return s
		}
		return s + blankN(need)
	}

	out := make([]string, height)
	for i := range out {
		out[i] = blankN(width)
	}
	if height < 5 || width == 0 {
		return out
	}

	// shared styles
	sepS  := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)
	warnS := lipgloss.NewStyle().Foreground(ColorWarn).Background(ColorBg)
	dimS  := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	kS    := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	vS    := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	// ── row 0: WHERE filter bar ───────────────────────────────────────────────
	{
		badge := warnS.Render("[WHERE]")
		input := dimS.Render(" d.state = 'Booted'")
		hints := strings.Join([]string{
			kS.Render("^F") + vS.Render(" filter"),
			kS.Render("^E") + vS.Render(" export"),
			kS.Render("a") + vS.Render(" add"),
			kS.Render("d") + vS.Render(" delete"),
		}, bgS.Render("  "))
		left := badge + input
		gap := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
		out[0] = fillTo(left + bgS.Render(strings.Repeat(" ", gap)) + hints)
	}

	// ── row 1: filter separator ───────────────────────────────────────────────
	out[1] = sepS.Render(strings.Repeat("─", width))

	// ── rows 2..h-3: bubbles table ────────────────────────────────────────────
	tableH := height - 4 // rows available for the table (header + data)
	nameW := max(width-fixedRenderedW-2, 15)
	p.tbl.SetColumns(resultsColumns(nameW))
	p.tbl.SetWidth(width)
	p.tbl.SetHeight(tableH)

	tableLines := strings.Split(p.tbl.View(), "\n")

	// The Cell style emits \x1b[0m resets that cancel the Selected background.
	// Fix: find the sentinel-tagged line, strip all ANSI, re-render at full width.
	selStyle := lipgloss.NewStyle().
		Background(ColorTableSelBg).
		Foreground(ColorFg).
		Width(width)
	for i, line := range tableLines {
		if strings.Contains(line, "\x1b[48;2;0;1;2m") { // sentinelBg escape
			tableLines[i] = selStyle.Render(ansi.Strip(line))
		}
	}

	for i := range tableH {
		if i < len(tableLines) {
			out[i+2] = fillTo(tableLines[i])
		}
	}

	// ── row h-2: pager separator ──────────────────────────────────────────────
	out[height-2] = sepS.Render(strings.Repeat("─", width))

	// ── row h-1: pager ───────────────────────────────────────────────────────
	{
		boldFgS := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
		btnS    := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBorder).Padding(0, 1)
		btnActS := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true).Padding(0, 1)

		total  := len(p.tbl.Rows())
		cursor := p.tbl.Cursor()

		count := boldFgS.Render(fmt.Sprintf("row %d", cursor+1))
		tot   := dimS.Render(fmt.Sprintf(" of %d", total))

		nav := strings.Join([]string{
			btnS.Render("⏮"),
			btnS.Render("◀"),
			btnActS.Render(fmt.Sprintf("%d", cursor+1)),
			btnS.Render("▶"),
			btnS.Render("⏭"),
		}, bgS.Render(" "))

		hints := strings.Join([]string{
			kS.Render("j/k") + vS.Render(" move"),
			kS.Render("g/G") + vS.Render(" top/end"),
		}, bgS.Render("  "))

		left := count + tot + bgS.Render("  ") + nav
		gap := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
		out[height-1] = fillTo(left + bgS.Render(strings.Repeat(" ", gap)) + hints)
	}

	return out
}
