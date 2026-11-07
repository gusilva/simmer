package dbviewer

import (
	"slices"
	"strings"
	"testing"

	"simmer/internal/device"
)

var testSchema = newSQLSchema([]device.SchemaTable{
	{Name: "orders", Columns: []string{"id", "user_id", "total"}},
	{Name: "users", Columns: []string{"id", "name", "created_at"}},
	{Name: "order items", Columns: []string{"order id", "qty"}},
	{Name: "recent_users", IsView: true, Columns: []string{"id", "name"}},
})

// at places the cursor at the "|" in text and returns the text without it
// and the cursor's line and rune column.
func at(text string) (string, int, int) {
	i := strings.Index(text, "|")
	before := text[:i]
	line := strings.Count(before, "\n")
	col := len([]rune(before[strings.LastIndex(before, "\n")+1:]))
	return before + text[i+1:], line, col
}

func labels(text string, manual bool) []string {
	t, line, col := at(text)
	var out []string
	for _, c := range sqlCompletions(t, line, col, testSchema, manual) {
		out = append(out, c.Type+":"+c.Text)
	}
	return out
}

func TestSQLCompletions(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		manual  bool
		want    []string // exactly, in order; nil means nothing offered
		include []string // at least these, when want is not given
		exclude []string
	}{
		{name: "tables after FROM", text: "SELECT * FROM u|",
			want: []string{"table:users"}},
		{name: "tables and views after FROM", text: "SELECT * FROM |", manual: true,
			want: []string{"table:orders", "table:users", `table:"order items"`, "view:recent_users"}},
		{name: "nothing right after FROM unless manual", text: "SELECT * FROM |"},
		{name: "tables after JOIN", text: "SELECT * FROM orders o JOIN us|",
			want: []string{"table:users"}},
		{name: "tables after a comma in FROM", text: "SELECT * FROM orders, us|",
			want: []string{"table:users"}},
		{name: "tables after INTO", text: "INSERT INTO or|",
			want: []string{"table:orders", `table:"order items"`}},
		{name: "tables after UPDATE", text: "UPDATE us|",
			want: []string{"table:users"}},
		{name: "columns after table dot", text: "SELECT users.|",
			want: []string{"column:id", "column:name", "column:created_at"}},
		{name: "columns after alias dot, alias defined later", text: "SELECT u.na| FROM users u",
			want: []string{"column:name"}},
		{name: "columns after AS alias", text: "SELECT o.| FROM orders AS o", manual: true,
			want: []string{"column:id", "column:user_id", "column:total"}},
		{name: "quoted column of a quoted table", text: `SELECT "order items".| FROM "order items"`,
			want: []string{`column:"order id"`, "column:qty"}},
		{name: "unknown qualifier", text: "SELECT x.| FROM users"},
		{name: "statement columns, functions and keywords", text: "SELECT na| FROM users",
			include: []string{"column:name"}, exclude: []string{"column:total"}},
		{name: "only the statement's tables", text: "SELECT * FROM orders;\nSELECT to| FROM users",
			exclude: []string{"column:total"}},
		{name: "keywords keep upper case", text: "SELECT * FROM users WH|",
			want: []string{"keyword:WHEN", "keyword:WHERE"}},
		{name: "keywords follow lower case", text: "select * from users wh|",
			want: []string{"keyword:when", "keyword:where"}},
		{name: "functions open a call", text: "SELECT cou|",
			want: []string{"function:count("}},
		{name: "nothing without a word unless manual", text: "SELECT * FROM users WHERE |"},
		{name: "manual offers everything", text: "SELECT * FROM users WHERE |", manual: true,
			include: []string{"column:name", "keyword:AND", "function:COUNT("}},
		{name: "exact match is not offered", text: "SELECT * FROM users|"},
		{name: "nothing inside a string", text: "SELECT * FROM users WHERE name = 'us|"},
		{name: "nothing inside a comment", text: "SELECT 1 -- FROM us|"},
		{name: "nothing inside a quoted identifier", text: `SELECT * FROM "us|`},
		{name: "a comma in a call is not a table list", text: "SELECT * FROM users WHERE coalesce(a, na|",
			include: []string{"column:name"}},
		{name: "a new statement after the last semicolon", text: "SELECT * FROM users;\nSELECT na|",
			exclude: []string{"column:name"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := labels(tc.text, tc.manual)
			if tc.include == nil && tc.exclude == nil {
				if !slices.Equal(got, tc.want) {
					t.Errorf("got  %v\nwant %v", got, tc.want)
				}
				return
			}
			for _, w := range tc.include {
				if !slices.Contains(got, w) {
					t.Errorf("missing %s in %v", w, got)
				}
			}
			for _, w := range tc.exclude {
				if slices.Contains(got, w) {
					t.Errorf("unexpected %s in %v", w, got)
				}
			}
		})
	}
}

func TestSchemaFromObjectsHasNamesOnly(t *testing.T) {
	s := schemaFromObjects(device.SQLiteObjects{
		Tables: []device.SchemaObject{{Name: "users", DisplayName: "People"}},
		Views:  []device.SchemaObject{{Name: "v"}},
	})
	items := sqlCompletions("SELECT * FROM ", 0, 14, s, true)
	var names []string
	for _, c := range items {
		names = append(names, c.Type+":"+c.Text)
	}
	if !slices.Equal(names, []string{"table:users", "view:v"}) {
		t.Errorf("got %v, want the real names, never display labels", names)
	}
}

func TestQuoteIdent(t *testing.T) {
	for in, want := range map[string]string{
		"users": "users", "_x1$": "_x1$", "order items": `"order items"`,
		"1st": `"1st"`, "select": `"select"`, `a"b`: `"a""b"`,
	} {
		if got := quoteIdent(in); got != want {
			t.Errorf("quoteIdent(%q) = %q, want %q", in, got, want)
		}
	}
}
