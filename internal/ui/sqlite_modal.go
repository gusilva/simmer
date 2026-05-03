package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"simmer/internal/device"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ShowSQLiteViewerMsg is dispatched when the user presses Enter on a .db file
// in the Android Files tab.
type ShowSQLiteViewerMsg struct {
	Device    device.Device
	PackageID string
	DBPath    string
	DBName    string
}

// SQLiteResultMsg carries the result of an async sqlite3 query.
type SQLiteResultMsg struct {
	Rows [][]string
	Err  error
}

type sqliteMode int

const (
	sqliteModeEdit sqliteMode = iota
	sqliteModeNav
	sqliteModeSearch
)

// SQLiteModal is an overlay for running SQLite queries against an Android app
// database via adb shell run-as.
type SQLiteModal struct {
	queryInput textinput.Model
	device     device.Device
	packageID  string
	dbPath     string
	dbName     string

	// rows[0] is the header row (sqlite3 -header), rows[1:] are data rows.
	rows        [][]string
	queryErr    string
	loading     bool
	rowOffset   int    // index of first visible row in filtered set
	cursor      int    // index of selected row in filtered set
	colOffset   int    // horizontal character scroll offset
	searchQuery string // active fuzzy filter
	mode        sqliteMode
	width       int // terminal width
	height      int // terminal height
}

const sqliteInitialQuery = "SELECT name FROM sqlite_master WHERE type='table';"

// NewSQLiteModal creates a modal pre-loaded with the default query and returns
// the cmd that runs the initial query automatically.
func NewSQLiteModal(dev device.Device, packageID, dbPath, dbName string) (SQLiteModal, tea.Cmd) {
	ti := textinput.New()
	ti.SetValue(sqliteInitialQuery)
	ti.CharLimit = 0
	ti.SetWidth(60)
	focusCmd := ti.Focus()

	m := SQLiteModal{
		queryInput: ti,
		device:     dev,
		packageID:  packageID,
		dbPath:     dbPath,
		dbName:     dbName,
		loading:    true,
		mode:       sqliteModeEdit,
	}
	return m, tea.Batch(focusCmd, m.buildQueryCmd())
}

// SetSize updates terminal dimensions and rescales the query input.
func (m *SQLiteModal) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.queryInput.SetWidth(m.innerW())
}

// SetResult stores the query result and clears the loading flag.
func (m *SQLiteModal) SetResult(rows [][]string, err error) {
	m.loading = false
	m.rowOffset = 0
	m.cursor = 0
	m.colOffset = 0
	m.searchQuery = ""
	if err != nil {
		m.queryErr = err.Error()
		m.rows = nil
	} else {
		m.queryErr = ""
		m.rows = rows
	}
}

// Update handles keyboard and paste input for the modal.
func (m SQLiteModal) Update(msg tea.Msg) (SQLiteModal, tea.Cmd) {
	// Forward paste events to the query input when in edit mode.
	if _, ok := msg.(tea.PasteMsg); ok {
		if m.mode == sqliteModeEdit {
			var cmd tea.Cmd
			m.queryInput, cmd = m.queryInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch m.mode {
	case sqliteModeEdit:
		switch k.String() {
		case "esc":
			m.mode = sqliteModeNav
			m.queryInput.Blur()
		case "enter":
			m.loading = true
			m.rowOffset = 0
			m.cursor = 0
			return m, m.buildQueryCmd()
		default:
			var cmd tea.Cmd
			m.queryInput, cmd = m.queryInput.Update(msg)
			return m, cmd
		}

	case sqliteModeSearch:
		switch k.String() {
		case "esc", "enter":
			m.mode = sqliteModeNav
		case "backspace":
			if len(m.searchQuery) > 0 {
				runes := []rune(m.searchQuery)
				m.searchQuery = string(runes[:len(runes)-1])
				m.resetCursor()
			}
		default:
			if k.Text != "" {
				m.searchQuery += k.Text
				m.resetCursor()
			}
		}

	case sqliteModeNav:
		switch k.String() {
		case "esc", "q":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.resetCursor()
				return m, nil
			}
			return m, func() tea.Msg { return CancelOverlayMsg{} }
		case "i", "e":
			m.mode = sqliteModeEdit
			return m, m.queryInput.Focus()
		case "/":
			m.mode = sqliteModeSearch
		case "enter":
			m.loading = true
			m.rowOffset = 0
			m.cursor = 0
			m.colOffset = 0
			m.searchQuery = ""
			return m, m.buildQueryCmd()
		case "left":
			step := max(m.innerW()/4, 4)
			m.colOffset = max(m.colOffset-step, 0)
		case "right":
			step := max(m.innerW()/4, 4)
			m.colOffset = min(m.colOffset+step, m.hScrollMax())
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.rowOffset {
					m.rowOffset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < m.filteredCount()-1 {
				m.cursor++
				if m.cursor >= m.rowOffset+m.visibleRows() {
					m.rowOffset = m.cursor - m.visibleRows() + 1
				}
			}
		case "space":
			if row := m.selectedRow(); row != nil {
				return m, CopyToClipboardCmd(strings.Join(row, "\t"))
			}
		}
	}

	return m, nil
}

// View renders the SQLite modal as an ANSI string for overlay placement.
func (m SQLiteModal) View() string {
	mw := m.modalW()
	innerW := m.innerW()

	platformColor := ColorAndroid
	if m.device.Platform == device.PlatformIOS {
		platformColor = ColorIOS
	}
	accent := lipgloss.NewStyle().Foreground(platformColor).Background(ColorBg).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	rule := faint.Render(strings.Repeat("─", innerW))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(platformColor).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder

	// Title: file name
	b.WriteString(accent.Render(truncateName("▲ "+m.dbName, innerW)))
	b.WriteString("\n\n")

	// Query input
	modeTag := ""
	if m.mode == sqliteModeEdit {
		modeTag = "  " + lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Render("[edit]")
	}
	b.WriteString(dim.Render("Query") + modeTag)
	b.WriteString("\n")
	b.WriteString(m.queryInput.View())
	b.WriteString("\n\n")

	// Results section
	b.WriteString(m.renderResults(innerW, dim, faint, rule))

	b.WriteString("\n\n")

	// Hints
	sep := faint.Render("  ")
	var hints []string
	switch m.mode {
	case sqliteModeEdit:
		hints = []string{
			hintKey.Render("Enter") + hintVerb.Render(" run"),
			hintKey.Render("Esc") + hintVerb.Render(" nav mode"),
		}
	case sqliteModeSearch:
		hints = []string{
			hintKey.Render("Enter/Esc") + hintVerb.Render(" done"),
		}
	default:
		hints = []string{
			hintKey.Render("i") + hintVerb.Render(" edit"),
			hintKey.Render("Enter") + hintVerb.Render(" run"),
			hintKey.Render("j/k") + hintVerb.Render(" scroll"),
			hintKey.Render("/") + hintVerb.Render(" search"),
			hintKey.Render("Space") + hintVerb.Render(" copy row"),
			hintKey.Render("Esc") + hintVerb.Render(" close"),
		}
		if m.hasHScroll() {
			hints = append(hints[:3:3],
				append([]string{hintKey.Render("←→") + hintVerb.Render(" h-scroll")}, hints[3:]...)...)
		}
	}
	b.WriteString(strings.Join(hints, sep))

	return box.Width(mw).Render(b.String())
}

func (m SQLiteModal) renderResults(innerW int, dim, faint lipgloss.Style, rule string) string {
	var b strings.Builder
	errStyle := lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg)
	visN := m.visibleRows()
	blank := strings.Repeat(" ", innerW)

	// writeDataArea always emits exactly visN data lines + 1 scroll-hint line.
	writeDataArea := func(lines []string, scrollHint string) {
		for i := 0; i < visN; i++ {
			if i < len(lines) {
				b.WriteString(lines[i])
			} else {
				b.WriteString(blank)
			}
			b.WriteString("\n")
		}
		b.WriteString(scrollHint) // no trailing newline — caller owns it
	}

	// Fixed frame: count(1) + search(1) + rule(1) + header(1) + rule(1) + data(visN) + hint(1)
	// Early-return states fill the same frame with blank lines.

	writeEmptyFrame := func(statusLine string) string {
		b.WriteString(statusLine)
		b.WriteString("\n")
		b.WriteString(m.renderSearchBar(innerW, faint))
		b.WriteString("\n")
		b.WriteString(rule)
		b.WriteString("\n")
		b.WriteString(blank) // header placeholder
		b.WriteString("\n")
		b.WriteString(rule)
		b.WriteString("\n")
		writeDataArea(nil, blank)

		return b.String()
	}

	if m.loading {
		return writeEmptyFrame(dim.Render("Results") + "  " + faint.Render("running…"))
	}

	if m.queryErr != "" {
		b.WriteString(dim.Render("Results") + "  " + errStyle.Render("error"))
		b.WriteString("\n")
		b.WriteString(m.renderSearchBar(innerW, faint))
		b.WriteString("\n")
		b.WriteString(rule)
		b.WriteString("\n")
		b.WriteString(blank)
		b.WriteString("\n")
		b.WriteString(rule)
		b.WriteString("\n")

		errLines := []string{errStyle.Render(truncateName(m.queryErr, innerW))}
		writeDataArea(errLines, blank)

		return b.String()
	}

	if len(m.rows) == 0 {
		return writeEmptyFrame(dim.Render("Results") + "  " + faint.Render("(no output)"))
	}

	header := m.rows[0]
	filtered := m.filteredData()
	nFiltered := len(filtered)
	nTotal := len(m.rows) - 1

	// Count line
	var countStr string
	if m.searchQuery != "" {
		countStr = fmt.Sprintf("%d/%d rows", nFiltered, nTotal)
	} else {
		countStr = fmt.Sprintf("%d row", nTotal)
		if nTotal != 1 {
			countStr += "s"
		}
	}
	b.WriteString(dim.Render("Results") + "  " + faint.Render(countStr))
	b.WriteString("\n")

	// Search bar
	b.WriteString(m.renderSearchBar(innerW, faint))
	b.WriteString("\n")

	// Decide between natural (H-scroll) and shrink-to-fit column widths.
	naturalWidths := sqliteNaturalColWidths(m.rows)
	fullW := sqliteRowWidth(naturalWidths)
	hScroll := fullW > innerW
	var widths []int
	if hScroll {
		widths = naturalWidths
	} else {
		widths = sqliteColWidths(m.rows, innerW)
	}

	// Clamp colOffset to valid range.
	maxOff := max(fullW-innerW, 0)
	colOff := min(m.colOffset, maxOff)

	renderTableRow := func(cells []string) string {
		full := sqliteRenderRowFull(cells, widths)
		if hScroll {
			return sqliteSliceRow(full, colOff, innerW)
		}
		return full
	}

	headerStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	selectedStyle := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)

	b.WriteString(rule)
	b.WriteString("\n")
	b.WriteString(headerStyle.Render(renderTableRow(header)))
	b.WriteString("\n")
	b.WriteString(rule)
	b.WriteString("\n")

	// Build exactly visN rendered data lines.
	dataLines := make([]string, visN)
	if nFiltered == 0 && m.searchQuery != "" {
		dataLines[0] = faint.Render("no matches")
	} else {
		end := min(m.rowOffset+visN, nFiltered)
		for i, row := range filtered[m.rowOffset:end] {
			absIdx := m.rowOffset + i
			rendered := renderTableRow(row)
			if absIdx == m.cursor {
				dataLines[i] = selectedStyle.Render(rendered)
			} else {
				dataLines[i] = rowStyle.Render(rendered)
			}
		}
	}

	var hintParts []string
	if nFiltered > visN {
		end := min(m.rowOffset+visN, nFiltered)
		hintParts = append(hintParts, fmt.Sprintf("rows %d–%d of %d ↑↓", m.rowOffset+1, end, nFiltered))
	}

	if hScroll {
		hintParts = append(hintParts, fmt.Sprintf("col %d/%d ←→", colOff, maxOff))
	}
	scrollHint := blank
	if len(hintParts) > 0 {
		scrollHint = faint.Render(strings.Join(hintParts, "  "))
	}

	writeDataArea(dataLines, scrollHint)
	return b.String()
}

func (m SQLiteModal) renderSearchBar(innerW int, faint lipgloss.Style) string {
	searchActive := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	searchDim := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	prefix := faint.Render("/") + " "
	query := m.searchQuery
	cursor := ""
	if m.mode == sqliteModeSearch {
		cursor = searchActive.Render("█")
	}

	var text string
	if query == "" && m.mode != sqliteModeSearch {
		text = faint.Render("filter…")
	} else {
		text = searchDim.Render(query) + cursor
	}

	raw := prefix + text
	maxLen := innerW - 2
	if lipgloss.Width(raw) > maxLen {
		// Truncate query so prompt stays within bounds.
		runes := []rune(query)
		for lipgloss.Width(prefix+searchDim.Render(string(runes))+cursor) > maxLen && len(runes) > 0 {
			runes = runes[1:]
		}
		text = searchDim.Render(string(runes)) + cursor
		raw = prefix + text
	}
	return raw
}

// ── sizing ─────────────────────────────────────────────────────────────────

func (m SQLiteModal) modalW() int {
	if m.width == 0 {
		return 80
	}
	// Use full terminal width with a 1-char margin on each side.
	return max(m.width-2, 60)
}

func (m SQLiteModal) hScrollMax() int {
	if len(m.rows) == 0 {
		return 0
	}
	return max(sqliteRowWidth(sqliteNaturalColWidths(m.rows))-m.innerW(), 0)
}

func (m SQLiteModal) hasHScroll() bool {
	return m.hScrollMax() > 0
}

func (m SQLiteModal) innerW() int {
	return max(m.modalW()-6, 20)
}

func (m SQLiteModal) visibleRows() int {
	// Content overhead (inside box, excluding scrollable rows):
	//   title(1) blank(1) query-label(1) input(1) blank(1)
	//   results-label(1) search-bar(1) rule(1) header(1) rule(1) blank(1) blank(1) hints(1) = 13
	// box adds border(2) + padding top/bot(2) = 4
	if m.height == 0 {
		return 8
	}
	return max(m.height-4-13, 3)
}

func (m SQLiteModal) filteredData() [][]string {
	if len(m.rows) <= 1 {
		return nil
	}
	data := m.rows[1:]
	if m.searchQuery == "" {
		return data
	}
	out := make([][]string, 0, len(data))
	for _, row := range data {
		for _, cell := range row {
			if fuzzyMatch(m.searchQuery, cell) {
				out = append(out, row)
				break
			}
		}
	}
	return out
}

func (m SQLiteModal) filteredCount() int {
	return len(m.filteredData())
}

func (m SQLiteModal) selectedRow() []string {
	filtered := m.filteredData()
	if m.cursor < 0 || m.cursor >= len(filtered) {
		return nil
	}
	return filtered[m.cursor]
}

// resetCursor resets cursor and scroll offset to zero after a search query change.
func (m *SQLiteModal) resetCursor() {
	m.cursor = 0
	m.rowOffset = 0
}

// ── query command ──────────────────────────────────────────────────────────

func (m SQLiteModal) buildQueryCmd() tea.Cmd {
	query := strings.TrimSpace(m.queryInput.Value())
	if query == "" {
		return nil
	}
	dev := m.device
	pkg := m.packageID
	path := m.dbPath
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rows, err := device.QuerySQLite(ctx, dev, pkg, path, query)
		return SQLiteResultMsg{Rows: rows, Err: err}
	}
}

// ── table helpers ──────────────────────────────────────────────────────────

// sqliteNaturalColWidths returns the max content width per column without shrinking.
func sqliteNaturalColWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	ncols := 0
	for _, row := range rows {
		if len(row) > ncols {
			ncols = len(row)
		}
	}
	if ncols == 0 {
		return nil
	}
	widths := make([]int, ncols)
	for _, row := range rows {
		for i, cell := range row {
			if i < ncols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}

// sqliteRowWidth returns the total rendered width of a row given column widths.
func sqliteRowWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	total := (len(widths) - 1) * 3 // " │ " separators
	for _, w := range widths {
		total += w
	}
	return total
}

// sqliteRenderRowFull renders a row to its natural full width with no terminal cap.
func sqliteRenderRowFull(cells []string, widths []int) string {
	if len(widths) == 0 {
		return strings.Join(cells, " | ")
	}
	parts := make([]string, len(widths))
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		if len(cell) > w {
			if w > 1 {
				cell = cell[:w-1] + "…"
			} else {
				cell = "…"
			}
		}
		if pad := w - len(cell); pad > 0 {
			cell += strings.Repeat(" ", pad)
		}
		parts[i] = cell
	}
	return strings.Join(parts, " │ ")
}

// sqliteSliceRow returns a rune-safe horizontal window of a rendered row.
func sqliteSliceRow(row string, offset, width int) string {
	runes := []rune(row)
	n := len(runes)
	start := min(offset, n)
	end := min(start+width, n)
	visible := string(runes[start:end])
	if end-start < width {
		visible += strings.Repeat(" ", width-(end-start))
	}
	return visible
}

// sqliteColWidths computes per-column widths from all rows and shrinks the
// widest columns until the total (including " │ " separators) fits in maxW.
func sqliteColWidths(rows [][]string, maxW int) []int {
	if len(rows) == 0 {
		return nil
	}
	ncols := 0
	for _, row := range rows {
		if len(row) > ncols {
			ncols = len(row)
		}
	}
	if ncols == 0 {
		return nil
	}

	widths := make([]int, ncols)
	for _, row := range rows {
		for i, cell := range row {
			if i < ncols && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Separator overhead: (ncols-1) * 3 for " │ "
	total := (ncols - 1) * 3
	for _, w := range widths {
		total += w
	}

	for total > maxW {
		maxI, maxV := 0, widths[0]
		for i, w := range widths {
			if w > maxV {
				maxI, maxV = i, w
			}
		}
		if maxV <= 1 {
			break
		}
		widths[maxI]--
		total--
	}
	return widths
}

// sqliteRenderRow renders one table row, padding each cell to its column width
// and truncating with "…" when the cell is too long.
func sqliteRenderRow(cells []string, widths []int, maxW int) string {
	if len(widths) == 0 {
		return strings.Join(cells, " | ")
	}
	parts := make([]string, len(widths))
	for i, w := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		if len(cell) > w {
			if w > 1 {
				cell = cell[:w-1] + "…"
			} else {
				cell = "…"
			}
		}
		if pad := w - len(cell); pad > 0 {
			cell += strings.Repeat(" ", pad)
		}
		parts[i] = cell
	}
	row := strings.Join(parts, " │ ")
	if len(row) > maxW {
		return row[:maxW]
	}
	return row
}
