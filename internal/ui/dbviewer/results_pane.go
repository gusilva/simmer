package dbviewer

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"simmer/internal/theme"
)

// ResultsPane renders the query results below the horizontal divider.
type ResultsPane struct {
	tbl table.Model
	rh  renderHelpers
}

func resultsStyles() table.Styles {
	return table.Styles{
		Header: lipgloss.NewStyle().
			Background(theme.ColorTableHeader).
			Foreground(theme.ColorFg).
			Bold(true).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Foreground(theme.ColorFgDim).
			Padding(0, 1),
		Selected: lipgloss.NewStyle(),
	}
}

func newResultsPane() ResultsPane {
	t := table.New(
		table.WithColumns(defaultResultsColumns(20)),
		table.WithRows(defaultResultsRows()),
		table.WithStyles(resultsStyles()),
		table.WithFocused(false),
	)
	return ResultsPane{tbl: t, rh: newRenderHelpers()}
}

func (p ResultsPane) withHeight(h int) ResultsPane {
	p.tbl.SetHeight(max(h-4, 3))
	return p
}

func (p ResultsPane) Update(msg tea.Msg) (ResultsPane, tea.Cmd) {
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
func (p ResultsPane) Rows(width, height int, focused bool) []string {
	rh := p.rh
	fillTo := func(s string) string { return rh.FillTo(s, width) }

	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}
	if height < 5 || width == 0 {
		return out
	}

	warnS := lipgloss.NewStyle().Foreground(theme.ColorWarn).Background(theme.ColorBg)
	dimS  := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)
	kS    := lipgloss.NewStyle().Foreground(theme.ColorBorderHi).Background(theme.ColorBg).Bold(true)
	vS    := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)

	// row 0: WHERE filter bar
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

	out[1] = rh.Sep(width)

	tableH := height - 4
	nameW := max(width-fixedRenderedW-2, 15)
	p.tbl.SetColumns(defaultResultsColumns(nameW))
	p.tbl.SetWidth(width)
	p.tbl.SetHeight(tableH)

	tableLines := strings.Split(p.tbl.View(), "\n")

	selStyle := lipgloss.NewStyle().
		Background(theme.ColorTableSelBg).
		Foreground(theme.ColorFg).
		Width(width)
	selectedLine := p.tbl.Cursor() + 1
	if selectedLine < len(tableLines) {
		tableLines[selectedLine] = selStyle.Render(ansi.Strip(tableLines[selectedLine]))
	}

	for i := range tableH {
		if i < len(tableLines) {
			out[i+2] = fillTo(tableLines[i])
		}
	}

	out[height-2] = rh.Sep(width)

	{
		boldFgS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg).Bold(true)
		btnS    := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBorder).Padding(0, 1)
		btnActS := lipgloss.NewStyle().Foreground(theme.ColorBg).Background(theme.ColorAccent).Bold(true).Padding(0, 1)

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
