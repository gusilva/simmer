package dbviewer

import (
	"fmt"
	"strings"

	"simmer/internal/theme"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ResultsPane renders the query results below the horizontal divider.
type ResultsPane struct {
	tbl          table.Model
	rh           renderHelpers
	headers      []string
	dataRows     [][]string
	naturalW     int
	xOffset      int
	errMsg       string
	loading      bool
	hasQuery     bool
	lastWasSpace bool // double-tap space detection
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
		table.WithColumns([]table.Column{{Title: "—", Width: 20}}),
		table.WithRows([]table.Row{}),
		table.WithStyles(resultsStyles()),
		table.WithFocused(false),
	)
	return ResultsPane{tbl: t, rh: newRenderHelpers()}
}

func (p ResultsPane) withHeight(h int) ResultsPane {
	p.tbl.SetHeight(max(h-4, 3))
	return p
}

// SetLoading marks the pane as waiting for a query result.
func (p *ResultsPane) SetLoading() {
	p.loading = true
	p.errMsg = ""
	p.headers = nil
	p.dataRows = nil
	p.naturalW = 0
	p.xOffset = 0
	p.hasQuery = true
	p.tbl.SetRows([]table.Row{})
}

// SetResults loads query output into the table. rows[0] is the header row.
func (p *ResultsPane) SetResults(rows [][]string) {
	p.loading = false
	p.errMsg = ""
	p.hasQuery = true
	p.xOffset = 0
	if len(rows) == 0 {
		p.headers = nil
		p.dataRows = nil
		p.naturalW = 0
		p.tbl.SetRows([]table.Row{})
		return
	}
	p.headers = rows[0]
	if len(rows) > 1 {
		p.dataRows = rows[1:]
	} else {
		p.dataRows = nil
	}
	cols := naturalResultColumns(p.headers, p.dataRows)
	p.naturalW = naturalTableWidth(cols)
	// Columns must be set before rows.
	p.tbl.SetColumns(cols)
	tableRows := make([]table.Row, len(p.dataRows))
	for i, r := range p.dataRows {
		tableRows[i] = table.Row(r)
	}
	p.tbl.SetRows(tableRows)
	p.tbl.GotoTop()
}

// SetError displays an error message in the pane.
func (p *ResultsPane) SetError(err error) {
	p.loading = false
	p.errMsg = err.Error()
	p.hasQuery = true
	p.headers = nil
	p.dataRows = nil
	p.naturalW = 0
	p.xOffset = 0
	p.tbl.SetRows([]table.Row{})
}

// naturalResultColumns computes column widths based on actual content — no truncation.
func naturalResultColumns(headers []string, dataRows [][]string) []table.Column {
	n := len(headers)
	if n == 0 {
		return []table.Column{{Title: "—", Width: 20}}
	}
	widths := make([]int, n)
	for i, h := range headers {
		widths[i] = max(len(h), 4)
	}
	for _, row := range dataRows {
		for i := 0; i < n && i < len(row); i++ {
			if l := len(row[i]); l > widths[i] {
				widths[i] = l
			}
		}
	}
	cols := make([]table.Column, n)
	for i, h := range headers {
		cols[i] = table.Column{Title: h, Width: widths[i]}
	}
	return cols
}

// naturalTableWidth sums all column widths plus 2-cell padding per column.
func naturalTableWidth(cols []table.Column) int {
	w := 0
	for _, c := range cols {
		w += c.Width + 2 // padding(0,1) = 1 left + 1 right
	}
	return w
}

func (p ResultsPane) Update(msg tea.Msg) (ResultsPane, tea.Cmd) {
	p.tbl.Focus()

	if k, ok := msg.(tea.KeyPressMsg); ok {
		key := k.String()
		if key != "space" {
			p.lastWasSpace = false
		}
		switch key {
		case "j", "down":
			p.tbl.MoveDown(1)
			return p, nil
		case "k", "up":
			p.tbl.MoveUp(1)
			return p, nil
		case "g":
			p.tbl.GotoTop()
			return p, nil
		case "G":
			p.tbl.GotoBottom()
			return p, nil
		case "h", "left":
			p.xOffset = max(p.xOffset-4, 0)
			return p, nil
		case "l", "right":
			p.xOffset += 4
			return p, nil
		case "H":
			p.xOffset = 0
			return p, nil
		case "L":
			p.xOffset = max(p.naturalW-1, 0)
			return p, nil
		case "w":
			p.xOffset = p.nextColOffset(p.xOffset)
			return p, nil
		case "b":
			p.xOffset = p.prevColOffset(p.xOffset)
			return p, nil
		case "space":
			if p.lastWasSpace {
				p.lastWasSpace = false
				return p, tea.SetClipboard(p.tableToTSV())
			}
			p.lastWasSpace = true
			return p, tea.SetClipboard(p.rowToTSV(p.tbl.Cursor()))
		}
	}

	var cmd tea.Cmd
	p.tbl, cmd = p.tbl.Update(msg)
	return p, cmd
}

// rowToTSV returns the tab-separated values of the row at the given index.
func (p ResultsPane) rowToTSV(idx int) string {
	if idx < 0 || idx >= len(p.dataRows) {
		return ""
	}
	return strings.Join(p.dataRows[idx], "\t")
}

// colOffsets returns the visual start position of each column.
func (p ResultsPane) colOffsets() []int {
	cols := naturalResultColumns(p.headers, p.dataRows)
	offsets := make([]int, len(cols))
	pos := 0
	for i, c := range cols {
		offsets[i] = pos
		pos += c.Width + 2 // padding(0,1) = 1 left + 1 right
	}
	return offsets
}

// nextColOffset returns the start of the column after the one at xOffset.
func (p ResultsPane) nextColOffset(xOffset int) int {
	for _, off := range p.colOffsets() {
		if off > xOffset {
			return off
		}
	}
	return max(p.naturalW-1, 0)
}

// prevColOffset returns the start of the column before the one at xOffset.
func (p ResultsPane) prevColOffset(xOffset int) int {
	offsets := p.colOffsets()
	prev := 0
	for _, off := range offsets {
		if off >= xOffset {
			break
		}
		prev = off
	}
	return prev
}

// tableToTSV returns the full table as tab-separated values with a header row.
func (p ResultsPane) tableToTSV() string {
	if len(p.headers) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(strings.Join(p.headers, "\t"))
	for _, row := range p.dataRows {
		sb.WriteByte('\n')
		sb.WriteString(strings.Join(row, "\t"))
	}
	return sb.String()
}

// Rows returns height strings each exactly width visual cells wide.
//
// Layout:
//
//	row 1        filter separator ──────
//	rows 2..h-3  table.View() — header + scrollable data rows
//	row h-2      pager separator ──────
//	row h-1      pager
func (p ResultsPane) Rows(width, height int, focused bool) []string {
	rh := p.rh
	if focused {
		rh = rh.WithSepColor(theme.ColorBorderHi)
	}
	fillTo := func(s string) string { return rh.FillTo(s, width) }

	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}
	if height < 5 || width == 0 {
		return out
	}

	errS := lipgloss.NewStyle().Foreground(theme.ColorErr).Background(theme.ColorBg)
	faintS := lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	dimS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)

	out[1] = rh.Sep(width)

	tableH := height - 4

	switch {
	case p.loading:
		out[2] = fillTo(rh.BlankN(2) + faintS.Render("executing query…"))

	case p.errMsg != "":
		out[2] = fillTo(rh.BlankN(2) + errS.Render("Error: "+p.errMsg))

	case !p.hasQuery:
		out[2] = fillTo(rh.BlankN(2) + faintS.Render("Press F5 or Ctrl+Enter to run a query"))

	default:
		// Render the table at its natural width; we clip manually below.
		p.tbl.SetColumns(naturalResultColumns(p.headers, p.dataRows))
		p.tbl.SetHeight(tableH)
		p.tbl.SetWidth(max(p.naturalW, 1))

		tableLines := strings.Split(p.tbl.View(), "\n")

		// Apply selection highlight on the stripped line before clipping.
		selStyle := lipgloss.NewStyle().
			Background(theme.ColorTableSelBg).
			Foreground(theme.ColorFg)
		selectedLine := p.tbl.Cursor() + 1
		if selectedLine < len(tableLines) {
			tableLines[selectedLine] = selStyle.Render(ansi.Strip(tableLines[selectedLine]))
		}

		// Clamp xOffset so we never scroll past the content.
		xOff := min(p.xOffset, max(p.naturalW-width, 0))

		for i := range tableH {
			if i >= len(tableLines) {
				break
			}
			clipped := ansi.Cut(tableLines[i], xOff, xOff+width)
			out[i+2] = fillTo(clipped)
		}
	}

	out[height-2] = rh.Sep(width)

	{
		boldFgS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg).Bold(true)
		btnS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBorder).Padding(0, 1)
		btnActS := lipgloss.NewStyle().Foreground(theme.ColorBg).Background(theme.ColorAccent).Bold(true).Padding(0, 1)

		total := len(p.dataRows)
		cursor := p.tbl.Cursor()

		var count, tot string
		if p.hasQuery && !p.loading && p.errMsg == "" {
			count = boldFgS.Render(fmt.Sprintf("row %d", cursor+1))
			tot = dimS.Render(fmt.Sprintf(" of %d", total))
		} else {
			count = boldFgS.Render("—")
			tot = dimS.Render(" rows")
		}

		nav := strings.Join([]string{
			btnS.Render("⏮ "),
			btnS.Render("◀"),
			btnActS.Render(fmt.Sprintf("%d", cursor+1)),
			btnS.Render("▶"),
			btnS.Render("⏭ "),
		}, rh.BlankN(1))

		left := count + tot + rh.BlankN(2) + nav
		out[height-1] = fillTo(left)
	}

	return out
}
