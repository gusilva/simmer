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
	rh  dbRenderHelpers
}



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
		// Selection highlight is re-applied in Rows() by index after stripping
		// the ANSI resets that the Cell style emits.
		Selected: lipgloss.NewStyle(),
	}
}

func newDBResultsPane() DBResultsPane {
	t := table.New(
		table.WithColumns(defaultResultsColumns(20)),
		table.WithRows(defaultResultsRows()),
		table.WithStyles(resultsStyles()),
		table.WithFocused(false),
	)
	return DBResultsPane{tbl: t, rh: newDBRenderHelpers()}
}

// withHeight returns a copy calibrated to h available rows.
// Called from DBViewerModal.SetSize. Layout overhead: filter(1)+filterSep(1)+pagerSep(1)+pager(1)=4.
func (p DBResultsPane) withHeight(h int) DBResultsPane {
	p.tbl.SetHeight(max(h-4, 3))
	return p
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
	rh := p.rh
	fillTo := func(s string) string { return rh.FillTo(s, width) }

	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}
	if height < 5 || width == 0 {
		return out
	}

	// shared styles
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
		}, rh.BlankN(2))
		left := badge + input
		gap := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
		out[0] = fillTo(left + rh.BlankN(gap) + hints)
	}

	// ── row 1: filter separator ───────────────────────────────────────────────
	out[1] = rh.Sep(width)

	// ── rows 2..h-3: bubbles table ────────────────────────────────────────────
	tableH := height - 4 // rows available for the table (header + data)
	nameW := max(width-fixedRenderedW-2, 15)
	p.tbl.SetColumns(defaultResultsColumns(nameW))
	p.tbl.SetWidth(width)
	p.tbl.SetHeight(tableH)

	tableLines := strings.Split(p.tbl.View(), "\n")

	// The Cell style emits \x1b[0m resets that cancel any Selected background.
	// Re-apply the highlight by index: row 0 is the header, row cursor+1 is selected.
	selStyle := lipgloss.NewStyle().
		Background(ColorTableSelBg).
		Foreground(ColorFg).
		Width(width)
	selectedLine := p.tbl.Cursor() + 1 // +1 for the header row
	if selectedLine < len(tableLines) {
		tableLines[selectedLine] = selStyle.Render(ansi.Strip(tableLines[selectedLine]))
	}

	for i := range tableH {
		if i < len(tableLines) {
			out[i+2] = fillTo(tableLines[i])
		}
	}

	// ── row h-2: pager separator ──────────────────────────────────────────────
	out[height-2] = rh.Sep(width)

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
		}, rh.BlankN(1))

		hints := strings.Join([]string{
			kS.Render("j/k") + vS.Render(" move"),
			kS.Render("g/G") + vS.Render(" top/end"),
		}, rh.BlankN(2))

		left := count + tot + rh.BlankN(2) + nav
		gap := max(width-lipgloss.Width(left)-lipgloss.Width(hints), 1)
		out[height-1] = fillTo(left + rh.BlankN(gap) + hints)
	}

	return out
}
