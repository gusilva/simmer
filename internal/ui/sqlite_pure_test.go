package ui

import (
	"strings"
	"testing"
)

// ---------- sqliteNaturalColWidths ----------

func TestSQLiteNaturalColWidths_Empty(t *testing.T) {
	if got := sqliteNaturalColWidths(nil); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
	if got := sqliteNaturalColWidths([][]string{}); got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

func TestSQLiteNaturalColWidths_SingleRow(t *testing.T) {
	rows := [][]string{{"id", "name", "email"}}
	got := sqliteNaturalColWidths(rows)
	if len(got) != 3 {
		t.Fatalf("expected 3 widths, got %d", len(got))
	}
	if got[0] != 2 { // "id"
		t.Errorf("col 0: got %d, want 2", got[0])
	}
	if got[1] != 4 { // "name"
		t.Errorf("col 1: got %d, want 4", got[1])
	}
	if got[2] != 5 { // "email"
		t.Errorf("col 2: got %d, want 5", got[2])
	}
}

func TestSQLiteNaturalColWidths_MaxAcrossRows(t *testing.T) {
	rows := [][]string{
		{"id", "Alice"},
		{"user_id", "Bob"},
	}
	got := sqliteNaturalColWidths(rows)
	if got[0] != 7 { // "user_id"
		t.Errorf("col 0: got %d, want 7", got[0])
	}
	if got[1] != 5 { // "Alice"
		t.Errorf("col 1: got %d, want 5", got[1])
	}
}

func TestSQLiteNaturalColWidths_JaggedRows(t *testing.T) {
	rows := [][]string{
		{"a", "bb", "ccc"},
		{"dd"},
	}
	got := sqliteNaturalColWidths(rows)
	if len(got) != 3 {
		t.Fatalf("expected 3 widths, got %d", len(got))
	}
	if got[0] != 2 { // "dd"
		t.Errorf("col 0: got %d, want 2", got[0])
	}
}

// ---------- sqliteRowWidth ----------

func TestSQLiteRowWidth_Empty(t *testing.T) {
	if got := sqliteRowWidth(nil); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestSQLiteRowWidth_SingleCol(t *testing.T) {
	if got := sqliteRowWidth([]int{10}); got != 10 {
		t.Errorf("expected 10, got %d", got)
	}
}

func TestSQLiteRowWidth_MultipleColumns(t *testing.T) {
	// 3 cols of widths 5,5,5 with " │ " separators (3 chars each, 2 separators = 6)
	got := sqliteRowWidth([]int{5, 5, 5})
	want := 5 + 3 + 5 + 3 + 5 // 21
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

// ---------- sqliteRenderRowFull ----------

func TestSQLiteRenderRowFull_Basic(t *testing.T) {
	cells := []string{"Alice", "30"}
	widths := []int{5, 4}
	got := sqliteRenderRowFull(cells, widths)
	if !strings.Contains(got, "Alice") {
		t.Errorf("expected Alice in output, got %q", got)
	}
	if !strings.Contains(got, "30") {
		t.Errorf("expected 30 in output, got %q", got)
	}
	if !strings.Contains(got, "│") {
		t.Errorf("expected separator in output, got %q", got)
	}
}

func TestSQLiteRenderRowFull_CellPadding(t *testing.T) {
	cells := []string{"hi"}
	widths := []int{6}
	got := sqliteRenderRowFull(cells, widths)
	if len(got) != 6 {
		t.Errorf("expected len 6, got %d: %q", len(got), got)
	}
}

func TestSQLiteRenderRowFull_CellTruncated(t *testing.T) {
	cells := []string{"toolong"}
	widths := []int{4}
	got := sqliteRenderRowFull(cells, widths)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) != 4 {
		t.Errorf("expected 4 runes, got %d: %q", len([]rune(got)), got)
	}
}

func TestSQLiteRenderRowFull_EmptyWidths(t *testing.T) {
	cells := []string{"a", "b"}
	got := sqliteRenderRowFull(cells, nil)
	if got == "" {
		t.Error("expected non-empty output for nil widths")
	}
}

func TestSQLiteRenderRowFull_MissingCells(t *testing.T) {
	// Fewer cells than widths — extra cols filled with spaces.
	cells := []string{"only"}
	widths := []int{4, 5}
	got := sqliteRenderRowFull(cells, widths)
	if !strings.Contains(got, "│") {
		t.Errorf("expected separator, got %q", got)
	}
}

// ---------- sqliteSliceRow ----------

func TestSQLiteSliceRow_NoOffset(t *testing.T) {
	row := "hello world"
	got := sqliteSliceRow(row, 0, 5)
	if got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestSQLiteSliceRow_WithOffset(t *testing.T) {
	row := "hello world"
	got := sqliteSliceRow(row, 6, 5)
	if got != "world" {
		t.Errorf("got %q, want world", got)
	}
}

func TestSQLiteSliceRow_PadsShortResult(t *testing.T) {
	row := "hi"
	got := sqliteSliceRow(row, 0, 6)
	if len([]rune(got)) != 6 {
		t.Errorf("expected 6 runes, got %d: %q", len([]rune(got)), got)
	}
}

func TestSQLiteSliceRow_OffsetBeyondEnd(t *testing.T) {
	row := "hello"
	got := sqliteSliceRow(row, 100, 5)
	if len([]rune(got)) != 5 {
		t.Errorf("expected padding to width 5, got %q", got)
	}
}

// ---------- sqliteColWidths ----------

func TestSQLiteColWidths_Empty(t *testing.T) {
	if got := sqliteColWidths(nil, 80); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestSQLiteColWidths_FitsNaturally(t *testing.T) {
	rows := [][]string{{"ab", "cd"}}
	// Natural width: 2 + 3 + 2 = 7 (separator " │ " = 3)
	got := sqliteColWidths(rows, 80)
	if got[0] != 2 || got[1] != 2 {
		t.Errorf("expected [2,2], got %v", got)
	}
}

func TestSQLiteColWidths_ShrinksToFit(t *testing.T) {
	rows := [][]string{{"abcdefgh", "ijklmnop"}} // each 8 chars
	// Request very narrow width — should shrink.
	got := sqliteColWidths(rows, 10)
	total := sqliteRowWidth(got)
	if total > 10 {
		// Shrinking stops when all columns are width 1, so allow exceeding only if unavoidable.
		allOne := true
		for _, w := range got {
			if w > 1 {
				allOne = false
			}
		}
		if !allOne {
			t.Errorf("total width %d exceeds maxW 10 with non-minimal cols %v", total, got)
		}
	}
}

// ---------- truncateName (sidebar) ----------

func TestTruncateName_FitsWithinMax(t *testing.T) {
	got := truncateName("hello", 10)
	if got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestTruncateName_Truncated(t *testing.T) {
	got := truncateName("hello world", 6)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis, got %q", got)
	}
	if len([]rune(got)) > 6 {
		t.Errorf("exceeds max: %q", got)
	}
}

// ---------- SQLiteModal methods (same-package access) ----------

func newTestModal(rows [][]string) SQLiteModal {
	m := SQLiteModal{}
	m.SetResult(rows, nil)
	return m
}

func TestSQLiteModal_FilteredData_NoRows(t *testing.T) {
	m := SQLiteModal{}
	if got := m.filteredData(); got != nil {
		t.Errorf("expected nil for empty modal, got %v", got)
	}
}

func TestSQLiteModal_FilteredData_HeaderOnly(t *testing.T) {
	m := newTestModal([][]string{{"id", "name"}})
	if got := m.filteredData(); got != nil {
		t.Errorf("expected nil for header-only, got %v", got)
	}
}

func TestSQLiteModal_FilteredData_NoFilter(t *testing.T) {
	rows := [][]string{
		{"id", "name"},
		{"1", "Alice"},
		{"2", "Bob"},
	}
	m := newTestModal(rows)
	got := m.filteredData()
	if len(got) != 2 {
		t.Errorf("expected 2 data rows, got %d", len(got))
	}
}

func TestSQLiteModal_FilteredData_WithFilter(t *testing.T) {
	rows := [][]string{
		{"name"},
		{"Alice"},
		{"Bob"},
		{"Charlie"},
	}
	m := newTestModal(rows)
	// "bob" is not a subsequence of Alice or Charlie.
	m.searchQuery = "bob"
	got := m.filteredData()
	if len(got) != 1 {
		t.Errorf("expected 1 match for 'bob', got %d", len(got))
	}
	if got[0][0] != "Bob" {
		t.Errorf("expected Bob, got %v", got[0])
	}
}

func TestSQLiteModal_FilteredCount(t *testing.T) {
	rows := [][]string{{"col"}, {"a"}, {"b"}, {"c"}}
	m := newTestModal(rows)
	if m.filteredCount() != 3 {
		t.Errorf("expected 3, got %d", m.filteredCount())
	}
}

func TestSQLiteModal_SelectedRow_InRange(t *testing.T) {
	rows := [][]string{{"col"}, {"first"}, {"second"}}
	m := newTestModal(rows)
	m.cursor = 1
	row := m.selectedRow()
	if row == nil || row[0] != "second" {
		t.Errorf("expected second, got %v", row)
	}
}

func TestSQLiteModal_SelectedRow_OutOfRange(t *testing.T) {
	m := SQLiteModal{}
	if got := m.selectedRow(); got != nil {
		t.Errorf("expected nil for empty modal, got %v", got)
	}
}

func TestSQLiteModal_VisibleRows_ZeroHeight(t *testing.T) {
	m := SQLiteModal{}
	if m.visibleRows() <= 0 {
		t.Errorf("expected positive visible rows for zero height default, got %d", m.visibleRows())
	}
}

func TestSQLiteModal_VisibleRows_WithHeight(t *testing.T) {
	m := SQLiteModal{height: 40}
	got := m.visibleRows()
	if got <= 0 {
		t.Errorf("expected positive visible rows, got %d", got)
	}
}

func TestSQLiteModal_SetResult_ClearsState(t *testing.T) {
	m := SQLiteModal{cursor: 5, rowOffset: 3, colOffset: 10, searchQuery: "old"}
	m.SetResult([][]string{{"col"}, {"row1"}}, nil)
	if m.cursor != 0 || m.rowOffset != 0 || m.colOffset != 0 || m.searchQuery != "" {
		t.Errorf("state not cleared: cursor=%d rowOffset=%d colOffset=%d searchQuery=%q",
			m.cursor, m.rowOffset, m.colOffset, m.searchQuery)
	}
}

func TestSQLiteModal_SetResult_Error(t *testing.T) {
	m := SQLiteModal{}
	m.SetResult(nil, errorStr("query failed"))
	if m.queryErr == "" {
		t.Error("expected queryErr to be set")
	}
	if m.rows != nil {
		t.Error("expected rows to be nil on error")
	}
}

// errorStr implements error.
type errorStr string

func (e errorStr) Error() string { return string(e) }
