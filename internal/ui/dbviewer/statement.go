package dbviewer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// statement is one Statement in the editor buffer: the byte range it spans,
// terminating ";" included, and whether it holds any SQL beyond whitespace
// and comments.
type statement struct {
	start, end int // byte offsets into the buffer; end is exclusive
	// codeStart is the offset of the first byte that is neither whitespace
	// nor part of a comment, or end when there is none.
	codeStart int
}

func (s statement) hasCode() bool { return s.codeStart < s.end }

// splitStatements cuts text into Statements at every ";" that SQLite would
// treat as a terminator: not inside a string literal, a quoted identifier
// ("…", `…`, […]) or a comment, and not inside the BEGIN … END body of a
// CREATE TRIGGER. The returned ranges cover text without gaps.
func splitStatements(text string) []statement {
	var out []statement
	cur := statement{start: 0, codeStart: -1}

	// Trigger bookkeeping, reset for every Statement.
	var words []string // leading keywords, enough to recognise CREATE … TRIGGER
	isTrigger := false
	depth := 0 // BEGIN/CASE … END nesting inside a trigger body

	finish := func(end int) {
		cur.end = end
		if cur.codeStart < 0 {
			cur.codeStart = end
		}
		out = append(out, cur)
		cur = statement{start: end, codeStart: -1}
		words, isTrigger, depth = nil, false, 0
	}
	markCode := func(i int) {
		if cur.codeStart < 0 {
			cur.codeStart = i
		}
	}

	for i := 0; i < len(text); {
		c := text[i]
		switch {
		case c == '-' && strings.HasPrefix(text[i:], "--"):
			if nl := strings.IndexByte(text[i:], '\n'); nl >= 0 {
				i += nl + 1
			} else {
				i = len(text)
			}

		case c == '/' && strings.HasPrefix(text[i:], "/*"):
			if e := strings.Index(text[i+2:], "*/"); e >= 0 {
				i += 2 + e + 2
			} else {
				i = len(text)
			}

		case c == '\'' || c == '"' || c == '`' || c == '[':
			markCode(i)
			closer := c
			if c == '[' {
				closer = ']'
			}
			i = skipQuoted(text, i+1, closer)

		case c == ';':
			markCode(i)
			i++
			if depth > 0 {
				continue
			}
			finish(i)

		case isIdentStart(c):
			markCode(i)
			j := i + 1
			for j < len(text) && isIdentPart(text[j]) {
				j++
			}
			word := strings.ToUpper(text[i:j])
			i = j

			if len(words) < 4 {
				words = append(words, word)
				// CREATE [TEMP|TEMPORARY] TRIGGER
				if words[0] == "CREATE" && word == "TRIGGER" {
					isTrigger = true
				}
			}
			if !isTrigger {
				continue
			}
			switch word {
			case "BEGIN":
				depth++
			case "CASE":
				if depth > 0 {
					depth++
				}
			case "END":
				if depth > 0 {
					depth--
				}
			}

		default:
			r, size := utf8.DecodeRuneInString(text[i:])
			if !unicode.IsSpace(r) {
				markCode(i)
			}
			i += size
		}
	}
	if cur.start < len(text) || len(out) == 0 {
		finish(len(text))
	}
	return out
}

// skipQuoted returns the offset just past the closing quote of a literal or
// quoted identifier whose body starts at i. A doubled closer is an escaped
// one, except for […], which has no escape. An unterminated quote runs to the
// end of text.
func skipQuoted(text string, i int, closer byte) int {
	for i < len(text) {
		if text[i] != closer {
			i++
			continue
		}
		if closer != ']' && i+1 < len(text) && text[i+1] == closer {
			i += 2
			continue
		}
		return i + 1
	}
	return len(text)
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9' || c == '$'
}

// statementAt returns the Statement under the cursor, given as a 0-indexed
// line and a rune column within it, trimmed of surrounding whitespace.
//
// A cursor after a Statement's ";" on the same line — in trailing spaces or a
// trailing comment — still belongs to that Statement, so "SELECT 1;|" runs
// SELECT 1. A cursor anywhere else outside SQL (a blank line between two
// Statements, the end of the buffer) picks the next Statement, or the last one
// when nothing follows.
func statementAt(text string, line, col int) string {
	stmts := splitStatements(text)
	off := byteOffset(text, line, col)

	idx := len(stmts) - 1
	for i, s := range stmts {
		if off < s.end || (off == s.end && i == len(stmts)-1) {
			idx = i
			break
		}
	}

	// Cursor in the run-up to this Statement's SQL, still on the line where
	// the previous Statement ended: that one is meant.
	s := stmts[idx]
	if idx > 0 && off < s.codeStart && !strings.Contains(text[s.start:off], "\n") {
		idx--
	}

	pick := -1
	for i := idx; i < len(stmts); i++ {
		if stmts[i].hasCode() {
			pick = i
			break
		}
	}
	if pick < 0 {
		for i := idx; i >= 0; i-- {
			if stmts[i].hasCode() {
				pick = i
				break
			}
		}
	}
	if pick < 0 {
		return ""
	}
	return strings.TrimSpace(text[stmts[pick].start:stmts[pick].end])
}

// byteOffset converts a 0-indexed line and rune column to a byte offset in
// text, clamping both to the text.
func byteOffset(text string, line, col int) int {
	off := 0
	for range max(line, 0) {
		nl := strings.IndexByte(text[off:], '\n')
		if nl < 0 {
			return len(text)
		}
		off += nl + 1
	}
	for col > 0 && off < len(text) && text[off] != '\n' {
		_, size := utf8.DecodeRuneInString(text[off:])
		off += size
		col--
	}
	return off
}
