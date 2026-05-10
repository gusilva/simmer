package dbviewer

import (
	"strings"
	"testing"
)

// ---------- statementAtLine ----------

func TestStatementAtLine_SingleStatement(t *testing.T) {
	text := "SELECT * FROM users;"
	got := statementAtLine(text, 0)
	if got != "SELECT * FROM users;" {
		t.Errorf("got %q", got)
	}
}

func TestStatementAtLine_MultipleStatements(t *testing.T) {
	text := "SELECT 1;\nSELECT 2;\nSELECT 3;"
	tests := []struct {
		line int
		want string
	}{
		{0, "SELECT 1;"},
		{1, "SELECT 2;"},
		{2, "SELECT 3;"},
	}
	for _, tc := range tests {
		got := statementAtLine(text, tc.line)
		if got != tc.want {
			t.Errorf("line %d: got %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestStatementAtLine_MultilineStatement(t *testing.T) {
	text := "SELECT *\nFROM users\nWHERE id = 1;"
	// All lines belong to the same statement.
	for line := 0; line <= 2; line++ {
		got := statementAtLine(text, line)
		if !strings.Contains(got, "SELECT") || !strings.Contains(got, "WHERE") {
			t.Errorf("line %d: missing full statement, got %q", line, got)
		}
	}
}

func TestStatementAtLine_CursorOnBlankLineBetweenStatements(t *testing.T) {
	// Blank line between two statements — cursor picks up the next statement.
	text := "SELECT 1;\n\nSELECT 2;"
	got := statementAtLine(text, 1) // blank line
	// Should return either one of the surrounding statements (not empty).
	if got == "" {
		t.Errorf("expected non-empty statement for blank line cursor, got empty")
	}
}

func TestStatementAtLine_EmptyText(t *testing.T) {
	got := statementAtLine("", 0)
	if got != "" {
		t.Errorf("expected empty for empty text, got %q", got)
	}
}

func TestStatementAtLine_NegativeLine(t *testing.T) {
	text := "SELECT 1;"
	got := statementAtLine(text, -5)
	if got != "SELECT 1;" {
		t.Errorf("got %q", got)
	}
}

func TestStatementAtLine_LineExceedsLength(t *testing.T) {
	text := "SELECT 1;"
	got := statementAtLine(text, 999)
	if got != "SELECT 1;" {
		t.Errorf("got %q", got)
	}
}

func TestStatementAtLine_TrimsSurroundingWhitespace(t *testing.T) {
	text := "  SELECT 1;  "
	got := statementAtLine(text, 0)
	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, " ") {
		t.Errorf("result not trimmed: %q", got)
	}
}

// ---------- truncateLabel ----------

func TestTruncateLabel_FitsWithinBudget(t *testing.T) {
	if got := truncateLabel("hello", 10); got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestTruncateLabel_ExactBudget(t *testing.T) {
	if got := truncateLabel("hello", 5); got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestTruncateLabel_Truncated(t *testing.T) {
	got := truncateLabel("hello world", 7)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	// Result must not exceed budget in rune count.
	if len([]rune(got)) > 7 {
		t.Errorf("exceeds budget: %q (len %d)", got, len([]rune(got)))
	}
}

func TestTruncateLabel_BudgetOne(t *testing.T) {
	got := truncateLabel("hello", 1)
	if got != "…" {
		t.Errorf("got %q, want …", got)
	}
}

func TestTruncateLabel_BudgetZero(t *testing.T) {
	got := truncateLabel("hello", 0)
	if got != "…" {
		t.Errorf("got %q, want …", got)
	}
}

func TestTruncateLabel_Empty(t *testing.T) {
	if got := truncateLabel("", 5); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestTruncateLabel_Unicode(t *testing.T) {
	// "日本語" is 3 runes; budget 2 → truncate.
	got := truncateLabel("日本語", 2)
	runes := []rune(got)
	if len(runes) > 2 {
		t.Errorf("exceeds rune budget: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis, got %q", got)
	}
}

// ---------- computeLayout ----------

func TestComputeLayout_BodyHPositive(t *testing.T) {
	l := computeLayout(120, 40)
	if l.BodyH <= 0 {
		t.Errorf("BodyH should be positive, got %d", l.BodyH)
	}
}

func TestComputeLayout_PaneWidthsAddUp(t *testing.T) {
	l := computeLayout(120, 40)
	// SidebarW + 1 (divider) + RightW == InnerW
	total := l.SidebarW + 1 + l.RightW
	if total != l.InnerW {
		t.Errorf("widths don't add up: %d + 1 + %d = %d, want %d", l.SidebarW, l.RightW, total, l.InnerW)
	}
}

func TestComputeLayout_DivRowWithinBody(t *testing.T) {
	l := computeLayout(120, 40)
	if l.DivRow <= 0 || l.DivRow >= l.BodyH {
		t.Errorf("DivRow %d outside body [1, %d)", l.DivRow, l.BodyH)
	}
}

func TestComputeLayout_SmallTerminal(t *testing.T) {
	// Should not panic or return nonsensical negatives.
	l := computeLayout(10, 10)
	if l.ModalW <= 0 || l.ModalH <= 0 {
		t.Errorf("expected positive modal dims for small terminal, got %+v", l)
	}
}

func TestComputeLayout_MinimumTerminal(t *testing.T) {
	// Degenerate terminal; clamped to minimum.
	l := computeLayout(0, 0)
	if l.ModalW <= 0 || l.ModalH <= 0 {
		t.Errorf("expected clamped modal dims, got %+v", l)
	}
}

// ---------- stripExt ----------

func TestStripExt_WithExtension(t *testing.T) {
	if got := stripExt("query.sql"); got != "query" {
		t.Errorf("got %q, want query", got)
	}
}

func TestStripExt_WithoutExtension(t *testing.T) {
	if got := stripExt("noext"); got != "noext" {
		t.Errorf("got %q, want noext", got)
	}
}

func TestStripExt_DotAtStart(t *testing.T) {
	// Leading dot is not treated as extension separator (i=0 not > 0).
	if got := stripExt(".hidden"); got != ".hidden" {
		t.Errorf("got %q, want .hidden", got)
	}
}

func TestStripExt_MultipleDotsKeepsLast(t *testing.T) {
	if got := stripExt("archive.tar.gz"); got != "archive.tar" {
		t.Errorf("got %q, want archive.tar", got)
	}
}

// ---------- explorerNode.realNameOrLabel ----------

func TestRealNameOrLabel_WithRealName(t *testing.T) {
	n := &explorerNode{label: "display", realName: "actual"}
	if got := n.realNameOrLabel(); got != "actual" {
		t.Errorf("got %q, want actual", got)
	}
}

func TestRealNameOrLabel_FallsBackToLabel(t *testing.T) {
	n := &explorerNode{label: "display"}
	if got := n.realNameOrLabel(); got != "display" {
		t.Errorf("got %q, want display", got)
	}
}

// ---------- nodeMatchesFilter ----------

func TestNodeMatchesFilter_LabelMatch(t *testing.T) {
	n := &explorerNode{label: "users"}
	if !nodeMatchesFilter(n, "user") {
		t.Error("expected match for prefix of label")
	}
}

func TestNodeMatchesFilter_NoMatch(t *testing.T) {
	n := &explorerNode{label: "orders"}
	if nodeMatchesFilter(n, "xyz") {
		t.Error("expected no match")
	}
}

func TestNodeMatchesFilter_ChildMatch(t *testing.T) {
	child := &explorerNode{label: "id"}
	parent := &explorerNode{label: "orders", children: []*explorerNode{child}}
	if !nodeMatchesFilter(parent, "id") {
		t.Error("expected match via child")
	}
}

func TestNodeMatchesFilter_CaseInsensitive(t *testing.T) {
	n := &explorerNode{label: "Users"}
	if !nodeMatchesFilter(n, "users") {
		t.Error("expected case-insensitive match")
	}
}

// ---------- Explorer.allRoots ----------

func TestAllRoots_NoScriptRoot(t *testing.T) {
	e := newExplorer()
	e.roots = []*explorerNode{{label: "a"}, {label: "b"}}
	got := e.allRoots()
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestAllRoots_WithScriptRoot(t *testing.T) {
	e := newExplorer()
	e.roots = []*explorerNode{{label: "a"}}
	e.scriptRoot = &explorerNode{label: "scripts"}
	got := e.allRoots()
	if len(got) != 2 {
		t.Errorf("expected 2 (root + scriptRoot), got %d", len(got))
	}
	if got[len(got)-1].label != "scripts" {
		t.Error("scriptRoot should be last")
	}
}

// ---------- Explorer.visibleItems ----------

func TestVisibleItems_Empty(t *testing.T) {
	e := newExplorer()
	if got := e.visibleItems(); len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestVisibleItems_CollapsedRoot(t *testing.T) {
	child := &explorerNode{label: "child"}
	root := &explorerNode{label: "root", children: []*explorerNode{child}, expanded: false}
	e := newExplorer()
	e.roots = []*explorerNode{root}
	got := e.visibleItems()
	if len(got) != 1 {
		t.Errorf("expected 1 (root only), got %d", len(got))
	}
}

func TestVisibleItems_ExpandedRoot(t *testing.T) {
	child := &explorerNode{label: "child"}
	root := &explorerNode{label: "root", children: []*explorerNode{child}, expanded: true}
	e := newExplorer()
	e.roots = []*explorerNode{root}
	got := e.visibleItems()
	if len(got) != 2 {
		t.Errorf("expected 2 (root + child), got %d", len(got))
	}
	if got[1].indent != 1 {
		t.Errorf("child indent: got %d, want 1", got[1].indent)
	}
}

// ---------- Explorer.filteredVisibleItems ----------

func TestFilteredVisibleItems_NoFilter(t *testing.T) {
	root := &explorerNode{label: "users"}
	e := newExplorer()
	e.roots = []*explorerNode{root}
	got := e.filteredVisibleItems()
	if len(got) != 1 {
		t.Errorf("expected 1, got %d", len(got))
	}
}

func TestFilteredVisibleItems_MatchingFilter(t *testing.T) {
	r1 := &explorerNode{label: "users"}
	r2 := &explorerNode{label: "orders"}
	e := newExplorer()
	e.roots = []*explorerNode{r1, r2}
	e.filterQuery = "user"
	got := e.filteredVisibleItems()
	if len(got) != 1 || got[0].node.label != "users" {
		t.Errorf("expected only users, got %v", got)
	}
}

func TestFilteredVisibleItems_NoMatch(t *testing.T) {
	root := &explorerNode{label: "users"}
	e := newExplorer()
	e.roots = []*explorerNode{root}
	e.filterQuery = "xyz"
	got := e.filteredVisibleItems()
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

// ---------- Explorer.selectedNodeInfo ----------

func TestSelectedNodeInfo_Empty(t *testing.T) {
	e := newExplorer()
	node, parent := e.selectedNodeInfo()
	if node != nil {
		t.Error("expected nil node for empty explorer")
	}
	if parent != "" {
		t.Error("expected empty parent")
	}
}

func TestSelectedNodeInfo_ValidCursor(t *testing.T) {
	root := &explorerNode{label: "users", kind: nodeKindTable}
	e := newExplorer()
	e.roots = []*explorerNode{root}
	e.cursor = 0
	node, _ := e.selectedNodeInfo()
	if node == nil || node.label != "users" {
		t.Errorf("expected users node, got %v", node)
	}
}

func TestSelectedNodeInfo_CursorOutOfRange(t *testing.T) {
	root := &explorerNode{label: "users"}
	e := newExplorer()
	e.roots = []*explorerNode{root}
	e.cursor = 99
	node, _ := e.selectedNodeInfo()
	if node != nil {
		t.Error("expected nil for out-of-range cursor")
	}
}

// ---------- Explorer.SetLoading ----------

func TestExplorer_SetLoading_ClearsState(t *testing.T) {
	e := newExplorer()
	e.roots = []*explorerNode{{label: "a"}}
	e.cursor = 5
	e.filterQuery = "old"
	e.SetLoading()
	if !e.loading {
		t.Error("expected loading=true")
	}
	if e.roots != nil {
		t.Error("expected roots nil after SetLoading")
	}
	if e.cursor != 0 {
		t.Error("expected cursor reset to 0")
	}
	if e.filterQuery != "" {
		t.Error("expected filterQuery cleared")
	}
}
