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
