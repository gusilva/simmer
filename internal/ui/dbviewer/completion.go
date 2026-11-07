package dbviewer

import (
	"slices"
	"strings"

	"simmer/internal/device"

	"github.com/gusilva/goeditor/core"
)

// sqlSchema is what completion knows about the open database: its tables and
// views, with columns when the schema query could list them.
type sqlSchema struct {
	tables []device.SchemaTable
	byName map[string]int // lower-cased name → index into tables
	// labels are the display names a custom tables query gives some
	// tables, lower-cased real name → label. Completion matches and shows
	// them, but always inserts the real name.
	labels map[string]string
}

func newSQLSchema(tables []device.SchemaTable) sqlSchema {
	s := sqlSchema{tables: tables, byName: make(map[string]int, len(tables))}
	for i, t := range tables {
		s.byName[strings.ToLower(t.Name)] = i
	}
	return s
}

// schemaFromObjects is the fallback schema when columns could not be listed:
// table and view names only, from the explorer's own listing.
func schemaFromObjects(objs device.SQLiteObjects) sqlSchema {
	var tables []device.SchemaTable
	for _, o := range objs.Tables {
		tables = append(tables, device.SchemaTable{Name: o.Name})
	}
	for _, o := range objs.Views {
		tables = append(tables, device.SchemaTable{Name: o.Name, IsView: true})
	}
	return newSQLSchema(tables).withLabels(tableLabels(objs))
}

// tableLabels collects the display labels the explorer's listing carries,
// lower-cased real name → label; nil when the tables query sets none.
func tableLabels(objs device.SQLiteObjects) map[string]string {
	var labels map[string]string
	for _, group := range [][]device.SchemaObject{objs.Tables, objs.Views} {
		for _, o := range group {
			if o.DisplayName == "" || o.DisplayName == o.Name {
				continue
			}
			if labels == nil {
				labels = map[string]string{}
			}
			labels[strings.ToLower(o.Name)] = o.DisplayName
		}
	}
	return labels
}

func (s sqlSchema) withLabels(labels map[string]string) sqlSchema {
	s.labels = labels
	return s
}

// label is what a table is shown as: its display label when it has one.
func (s sqlSchema) label(name string) string {
	if l, ok := s.labels[strings.ToLower(name)]; ok {
		return l
	}
	return name
}

func (s sqlSchema) table(name string) (device.SchemaTable, bool) {
	i, ok := s.byName[strings.ToLower(name)]
	if !ok {
		return device.SchemaTable{}, false
	}
	return s.tables[i], true
}

// Completion kinds, also shown as the item's type in the menu.
const (
	kindColumn   = "column"
	kindTable    = "table"
	kindView     = "view"
	kindFunction = "function"
	kindKeyword  = "keyword"
)

// maxCompletions caps the menu; past this the prefix is too short to help.
const maxCompletions = 100

// sqlCompletions returns what may complete the word at the cursor (0-indexed
// line, rune column) in text. What is offered depends on where the cursor is
// in its Statement:
//
//   - after "x.": the columns of the table x names or aliases;
//   - after FROM, JOIN, INTO, UPDATE or TABLE (or a comma in a FROM list):
//     tables and views;
//   - anywhere else: columns of the tables the Statement uses, functions and
//     keywords — only once a word is started, unless manual.
//
// Nothing is offered inside a string, a quoted identifier or a comment.
func sqlCompletions(text string, line, col int, schema sqlSchema, manual bool) []core.Completion {
	off := byteOffset(text, line, col)
	stmt := statementAround(text, off)

	before, open := sqlTokens(text[stmt.start:off])
	if open {
		return nil
	}
	all, _ := sqlTokens(text[stmt.start:stmt.end])

	prefix := ""
	if n := len(before); n > 0 && before[n-1].kind == tokWord && before[n-1].end == off-stmt.start {
		prefix = before[n-1].text
		before = before[:n-1]
	}

	var items []core.Completion
	switch {
	case endsWithPunct(before, "."):
		if len(before) < 2 || !before[len(before)-2].isName() {
			return nil
		}
		qualifier := before[len(before)-2].value
		tbl, ok := schema.table(qualifier)
		if !ok {
			tbl, ok = schema.table(tableRefs(all)[strings.ToLower(qualifier)])
		}
		if !ok {
			return nil
		}
		items = columnItems(schema, tbl, prefix)

	case wantsTable(before):
		// Right after the keyword the menu waits for a letter, as elsewhere.
		if prefix == "" && !manual {
			return nil
		}
		items = tableItems(schema, prefix)

	default:
		if prefix == "" && !manual {
			return nil
		}
		seen := map[string]bool{}
		for _, name := range sortedValues(tableRefs(all)) {
			tbl, ok := schema.table(name)
			if !ok || seen[strings.ToLower(tbl.Name)] {
				continue
			}
			seen[strings.ToLower(tbl.Name)] = true
			items = append(items, columnItems(schema, tbl, prefix)...)
		}
		items = append(items, wordItems(sqlFunctions, kindFunction, prefix)...)
		items = append(items, wordItems(sqlKeywords, kindKeyword, prefix)...)
	}

	if len(items) > maxCompletions {
		items = items[:maxCompletions]
	}
	return items
}

// statementAround returns the Statement the byte offset off falls in. Past a
// final ";" the cursor starts a new, empty one.
func statementAround(text string, off int) statement {
	stmts := splitStatements(text)
	for _, s := range stmts {
		if off < s.end {
			return s
		}
	}
	last := stmts[len(stmts)-1]
	if strings.HasSuffix(strings.TrimSpace(text[last.start:last.end]), ";") {
		return statement{start: off, end: off, codeStart: off}
	}
	return last
}

func endsWithPunct(toks []sqlToken, p string) bool {
	return len(toks) > 0 && toks[len(toks)-1].kind == tokPunct && toks[len(toks)-1].text == p
}

// tableKeywords are followed by a table name.
var tableKeywords = map[string]bool{"FROM": true, "JOIN": true, "INTO": true, "UPDATE": true, "TABLE": true}

// clauseKeywords start a clause; the latest one says what a comma continues.
var clauseKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "GROUP": true, "ORDER": true,
	"HAVING": true, "LIMIT": true, "SET": true, "VALUES": true, "ON": true,
	"USING": true, "RETURNING": true, "JOIN": true,
}

// wantsTable reports whether the next word is a table name.
func wantsTable(before []sqlToken) bool {
	if len(before) == 0 {
		return false
	}
	last := before[len(before)-1]
	if last.kind == tokWord && tableKeywords[last.upper()] {
		return true
	}
	if !endsWithPunct(before, ",") {
		return false
	}
	// A comma inside FROM's table list, not inside a call's parentheses.
	depth := 0
	for i := len(before) - 2; i >= 0; i-- {
		t := before[i]
		switch {
		case t.kind == tokPunct && t.text == ")":
			depth++
		case t.kind == tokPunct && t.text == "(":
			if depth == 0 {
				return false
			}
			depth--
		case depth == 0 && t.kind == tokWord && clauseKeywords[t.upper()]:
			return t.upper() == "FROM"
		}
	}
	return false
}

// tableRefs maps every name a Statement uses for a table — the table's own
// name and any alias — to the table's name, lower-cased keys.
func tableRefs(toks []sqlToken) map[string]string {
	refs := map[string]string{}
	inFrom := false
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.kind == tokWord {
			up := t.upper()
			if clauseKeywords[up] || tableKeywords[up] {
				inFrom = up == "FROM"
			}
		}
		startsRef := t.kind == tokWord && tableKeywords[t.upper()] ||
			inFrom && t.kind == tokPunct && t.text == ","
		if !startsRef || i+1 >= len(toks) || !toks[i+1].isName() {
			continue
		}
		name := toks[i+1].value
		if toks[i+1].isKeyword() {
			continue // e.g. "INSERT INTO" … "SELECT", or a typo
		}
		refs[strings.ToLower(name)] = name
		j := i + 2
		if j < len(toks) && toks[j].kind == tokWord && toks[j].upper() == "AS" {
			j++
		}
		if j < len(toks) && toks[j].isName() && !toks[j].isKeyword() {
			refs[strings.ToLower(toks[j].value)] = name
		}
		i++
	}
	return refs
}

func sortedValues(m map[string]string) []string {
	var out []string
	for _, v := range m {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.Sort(out)
	return out
}

func matches(name, prefix string) bool {
	return len(name) > len(prefix) && strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}

func columnItems(s sqlSchema, t device.SchemaTable, prefix string) []core.Completion {
	var out []core.Completion
	for _, c := range t.Columns {
		if prefix == "" || matches(c, prefix) {
			out = append(out, core.Completion{
				Text: quoteIdent(c), Label: c, Type: kindColumn, Description: s.label(t.Name),
			})
		}
	}
	return out
}

// tableItems offers tables and views by what the explorer shows them as — a
// display label when the tables query gives one, matched as well as the real
// name — in that order, inserting the real name.
func tableItems(s sqlSchema, prefix string) []core.Completion {
	var out []core.Completion
	for _, t := range s.tables {
		shown := s.label(t.Name)
		if prefix != "" && !matches(shown, prefix) && !matches(t.Name, prefix) {
			continue
		}
		kind := kindTable
		if t.IsView {
			kind = kindView
		}
		item := core.Completion{Text: quoteIdent(t.Name), Label: shown, Type: kind}
		if shown != t.Name {
			item.Description = t.Name
		}
		out = append(out, item)
	}
	if s.labels != nil {
		slices.SortStableFunc(out, func(a, b core.Completion) int {
			return strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label))
		})
	}
	return out
}

// wordItems offers keywords or functions in the case the user is typing:
// lower-case after a lower-case prefix, upper-case otherwise.
func wordItems(words []string, kind, prefix string) []core.Completion {
	lower := prefix != "" && prefix == strings.ToLower(prefix)
	var out []core.Completion
	for _, w := range words {
		if prefix != "" && !matches(w, prefix) {
			continue
		}
		if lower {
			w = strings.ToLower(w)
		}
		item := core.Completion{Text: w, Label: w, Type: kind}
		if kind == kindFunction {
			item.Text = w + "("
			item.Label = w + "()"
		}
		out = append(out, item)
	}
	return out
}

// quoteIdent double-quotes a name that is not a plain identifier or that is a
// keyword, so the inserted text runs as written.
func quoteIdent(name string) string {
	plain := name != "" && !sqlKeywordSet[strings.ToUpper(name)]
	for i := 0; i < len(name) && plain; i++ {
		c := name[i]
		plain = c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && (c >= '0' && c <= '9' || c == '$')
	}
	if plain {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// ── tokens ───────────────────────────────────────────────────────────────────

type tokenKind int

const (
	tokWord   tokenKind = iota // keyword or bare identifier
	tokQuoted                  // "…", `…` or […] identifier
	tokString                  // '…' literal
	tokNumber
	tokPunct
)

type sqlToken struct {
	kind       tokenKind
	text       string // as written
	value      string // identifier without its quotes
	start, end int    // byte offsets
}

func (t sqlToken) upper() string { return strings.ToUpper(t.text) }
func (t sqlToken) isName() bool  { return t.kind == tokWord || t.kind == tokQuoted }

// isKeyword reports a bare word SQLite reserves, which cannot be an alias.
func (t sqlToken) isKeyword() bool { return t.kind == tokWord && sqlKeywordSet[t.upper()] }

// sqlTokens splits SQL into tokens, dropping whitespace and comments. open
// reports that s ends inside a string, quoted identifier or comment.
func sqlTokens(s string) (toks []sqlToken, open bool) {
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++

		case c == '-' && strings.HasPrefix(s[i:], "--"):
			nl := strings.IndexByte(s[i:], '\n')
			if nl < 0 {
				return toks, true
			}
			i += nl + 1

		case c == '/' && strings.HasPrefix(s[i:], "/*"):
			e := strings.Index(s[i+2:], "*/")
			if e < 0 {
				return toks, true
			}
			i += 2 + e + 2

		case c == '\'' || c == '"' || c == '`' || c == '[':
			closer := c
			if c == '[' {
				closer = ']'
			}
			j := skipQuoted(s, i+1, closer)
			if j == len(s) && (j-i < 2 || s[j-1] != closer) {
				return toks, true
			}
			body := s[i+1 : j-1]
			kind := tokQuoted
			if c == '\'' {
				kind = tokString
			} else if closer != ']' {
				body = strings.ReplaceAll(body, string([]byte{closer, closer}), string(closer))
			}
			toks = append(toks, sqlToken{kind: kind, text: s[i:j], value: body, start: i, end: j})
			i = j

		case isIdentStart(c):
			j := i + 1
			for j < len(s) && isIdentPart(s[j]) {
				j++
			}
			toks = append(toks, sqlToken{kind: tokWord, text: s[i:j], value: s[i:j], start: i, end: j})
			i = j

		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(s) && (isIdentPart(s[j]) || s[j] == '.') {
				j++
			}
			toks = append(toks, sqlToken{kind: tokNumber, text: s[i:j], start: i, end: j})
			i = j

		default:
			toks = append(toks, sqlToken{kind: tokPunct, text: s[i : i+1], start: i, end: i + 1})
			i++
		}
	}
	return toks, false
}

// ── vocabulary ───────────────────────────────────────────────────────────────

var sqlKeywords = []string{
	"ADD", "ALL", "ALTER", "ANALYZE", "AND", "AS", "ASC", "ATTACH", "BEGIN",
	"BETWEEN", "BY", "CASE", "CAST", "CHECK", "COLLATE", "COLUMN", "COMMIT",
	"CREATE", "CROSS", "DEFAULT", "DELETE", "DESC", "DETACH", "DISTINCT",
	"DROP", "ELSE", "END", "ESCAPE", "EXCEPT", "EXISTS", "EXPLAIN", "FALSE",
	"FOREIGN", "FROM", "FULL", "GLOB", "GROUP", "HAVING", "IF", "IN", "INDEX",
	"INNER", "INSERT", "INTERSECT", "INTO", "IS", "ISNULL", "JOIN", "KEY",
	"LEFT", "LIKE", "LIMIT", "NATURAL", "NOT", "NOTNULL", "NULL", "NULLS",
	"OFFSET", "ON", "OR", "ORDER", "OUTER", "PLAN", "PRAGMA", "PRIMARY",
	"QUERY", "RECURSIVE", "REFERENCES", "REGEXP", "RENAME", "REPLACE",
	"RETURNING", "RIGHT", "ROLLBACK", "SELECT", "SET", "TABLE", "THEN", "TO",
	"TRANSACTION", "TRIGGER", "TRUE", "UNION", "UNIQUE", "UPDATE", "USING",
	"VACUUM", "VALUES", "VIEW", "WHEN", "WHERE", "WITH",
}

var sqlKeywordSet = func() map[string]bool {
	m := make(map[string]bool, len(sqlKeywords))
	for _, k := range sqlKeywords {
		m[k] = true
	}
	return m
}()

var sqlFunctions = []string{
	"ABS", "AVG", "CHANGES", "CHAR", "COALESCE", "COUNT", "DATE", "DATETIME",
	"GROUP_CONCAT", "HEX", "IFNULL", "IIF", "INSTR", "JSON", "JSON_ARRAY",
	"JSON_EXTRACT", "JSON_OBJECT", "JULIANDAY", "LAST_INSERT_ROWID", "LENGTH",
	"LOWER", "LTRIM", "MAX", "MIN", "NULLIF", "PRINTF", "QUOTE", "RANDOM",
	"ROUND", "RTRIM", "STRFTIME", "SUBSTR", "SUM", "TIME", "TOTAL", "TRIM",
	"TYPEOF", "UNICODE", "UNIXEPOCH", "UPPER",
}
