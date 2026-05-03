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
	rows     [][]string
	queryErr string
	loading  bool
	rowOffset int
	mode     sqliteMode
	width    int // terminal width
	height   int // terminal height
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
	if err != nil {
		m.queryErr = err.Error()
		m.rows = nil
	} else {
		m.queryErr = ""
		m.rows = rows
	}
}

// Update handles keyboard input for the modal.
func (m SQLiteModal) Update(msg tea.Msg) (SQLiteModal, tea.Cmd) {
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
			return m, m.buildQueryCmd()
		default:
			var cmd tea.Cmd
			m.queryInput, cmd = m.queryInput.Update(msg)
			return m, cmd
		}

	case sqliteModeNav:
		switch k.String() {
		case "esc", "q":
			return m, func() tea.Msg { return CancelOverlayMsg{} }
		case "i", "e":
			m.mode = sqliteModeEdit
			return m, m.queryInput.Focus()
		case "enter":
			m.loading = true
			m.rowOffset = 0
			return m, m.buildQueryCmd()
		case "up", "k":
			if m.rowOffset > 0 {
				m.rowOffset--
			}
		case "down", "j":
			if m.rowOffset < max(m.dataRowCount()-m.visibleRows(), 0) {
				m.rowOffset++
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
	if m.mode == sqliteModeEdit {
		hints = []string{
			hintKey.Render("Enter") + hintVerb.Render(" run"),
			hintKey.Render("Esc") + hintVerb.Render(" nav mode"),
		}
	} else {
		hints = []string{
			hintKey.Render("i") + hintVerb.Render(" edit"),
			hintKey.Render("Enter") + hintVerb.Render(" run"),
			hintKey.Render("j/k") + hintVerb.Render(" scroll"),
			hintKey.Render("Esc") + hintVerb.Render(" close"),
		}
	}
	b.WriteString(strings.Join(hints, sep))

	return box.Width(mw).Render(b.String())
}

func (m SQLiteModal) renderResults(innerW int, dim, faint lipgloss.Style, rule string) string {
	var b strings.Builder
	errStyle := lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg)

	if m.loading {
		b.WriteString(dim.Render("Results") + "  " + faint.Render("running…"))
		b.WriteString("\n")
		b.WriteString(rule)
		return b.String()
	}

	if m.queryErr != "" {
		b.WriteString(dim.Render("Results") + "  " + errStyle.Render("error"))
		b.WriteString("\n")
		b.WriteString(rule)
		b.WriteString("\n")
		b.WriteString(errStyle.Render(truncateName(m.queryErr, innerW)))
		return b.String()
	}

	if len(m.rows) == 0 {
		b.WriteString(dim.Render("Results") + "  " + faint.Render("(no output)"))
		b.WriteString("\n")
		b.WriteString(rule)
		return b.String()
	}

	header := m.rows[0]
	data := m.rows[1:]
	nData := len(data)

	countStr := fmt.Sprintf("%d row", nData)
	if nData != 1 {
		countStr += "s"
	}
	b.WriteString(dim.Render("Results") + "  " + faint.Render(countStr))
	b.WriteString("\n")

	widths := sqliteColWidths(m.rows, innerW)

	headerStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	b.WriteString(rule)
	b.WriteString("\n")
	b.WriteString(headerStyle.Render(sqliteRenderRow(header, widths, innerW)))
	b.WriteString("\n")
	b.WriteString(rule)
	b.WriteString("\n")

	visN := m.visibleRows()
	end := min(m.rowOffset+visN, nData)
	for _, row := range data[m.rowOffset:end] {
		b.WriteString(rowStyle.Render(sqliteRenderRow(row, widths, innerW)))
		b.WriteString("\n")
	}

	if nData > visN {
		hint := fmt.Sprintf("rows %d–%d of %d  ↑↓ scroll", m.rowOffset+1, end, nData)
		b.WriteString(faint.Render(hint))
	}

	return b.String()
}

// ── sizing ─────────────────────────────────────────────────────────────────

func (m SQLiteModal) modalW() int {
	if m.width == 0 {
		return 80
	}
	return min(max(m.width-4, 60), 100)
}

func (m SQLiteModal) innerW() int {
	return max(m.modalW()-6, 20)
}

func (m SQLiteModal) visibleRows() int {
	// Content overhead (inside box, excluding scrollable rows):
	//   title(1) blank(1) query-label(1) input(1) blank(1)
	//   results-label(1) rule(1) header(1) rule(1) blank(1) blank(1) hints(1) = 12
	// box adds border(2) + padding top/bot(2) = 4
	if m.height == 0 {
		return 8
	}
	return max(m.height-4-12, 3)
}

func (m SQLiteModal) dataRowCount() int {
	if len(m.rows) <= 1 {
		return 0
	}
	return len(m.rows) - 1
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
