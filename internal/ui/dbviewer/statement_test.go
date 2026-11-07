package dbviewer

import "testing"

func TestStatementAt(t *testing.T) {
	trigger := "CREATE TRIGGER tr AFTER INSERT ON t BEGIN\n" +
		"  UPDATE t SET a = CASE WHEN 1 THEN 2 END;\n" +
		"  DELETE FROM u;\n" +
		"END;"

	tests := []struct {
		name      string
		text      string
		line, col int
		want      string
	}{
		{"two on one line, first", "SELECT 1; SELECT 2;", 0, 3, "SELECT 1;"},
		{"two on one line, second", "SELECT 1; SELECT 2;", 0, 12, "SELECT 2;"},
		{"cursor right after the semicolon", "SELECT 1;\nSELECT 2;", 0, 9, "SELECT 1;"},
		{"trailing spaces after the semicolon", "SELECT 1;   \nSELECT 2;", 0, 12, "SELECT 1;"},
		{"trailing comment after the semicolon", "SELECT 1; -- note\nSELECT 2;", 0, 15, "SELECT 1;"},
		{"blank line picks the next", "SELECT 1;\n\nSELECT 2;", 1, 0, "SELECT 2;"},
		{"end of buffer picks the last", "SELECT 1;\nSELECT 2;\n\n", 3, 0, "SELECT 2;"},
		{"unterminated last statement", "SELECT 1;\nSELECT 2", 1, 3, "SELECT 2"},
		{"semicolon in a string", "SELECT 'a;b' FROM t;\nSELECT 2;", 0, 0, "SELECT 'a;b' FROM t;"},
		{"escaped quote in a string", "SELECT 'it''s;' FROM t;", 0, 0, "SELECT 'it''s;' FROM t;"},
		{"semicolon in a double-quoted identifier", `SELECT "a;b" FROM t;`, 0, 0, `SELECT "a;b" FROM t;`},
		{"semicolon in a backtick identifier", "SELECT `a;b` FROM t;", 0, 0, "SELECT `a;b` FROM t;"},
		{"semicolon in a bracketed identifier", "SELECT [a;b] FROM t;", 0, 0, "SELECT [a;b] FROM t;"},
		{"semicolon in a line comment", "SELECT 1 -- x;\nFROM t;", 0, 0, "SELECT 1 -- x;\nFROM t;"},
		{"semicolon in a block comment", "SELECT /* x; */ 1;", 0, 0, "SELECT /* x; */ 1;"},
		{"trigger body is one statement", trigger + "\nSELECT 2;", 2, 4, trigger},
		{"after a trigger", trigger + "\nSELECT 2;", 4, 3, "SELECT 2;"},
		{"temp trigger", "CREATE TEMP TRIGGER tr BEFORE DELETE ON t BEGIN SELECT 1; END;", 0, 0,
			"CREATE TEMP TRIGGER tr BEFORE DELETE ON t BEGIN SELECT 1; END;"},
		{"BEGIN transaction is not a body", "BEGIN;\nSELECT 1;", 0, 2, "BEGIN;"},
		{"after a BEGIN transaction", "BEGIN;\nSELECT 1;", 1, 2, "SELECT 1;"},
		{"rune column on a multibyte line", "SELECT 'é'; SELECT 2;", 0, 12, "SELECT 2;"},
		{"only comments", "-- nothing here", 0, 3, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := statementAt(tc.text, tc.line, tc.col); got != tc.want {
				t.Errorf("statementAt(%q, %d, %d) = %q, want %q", tc.text, tc.line, tc.col, got, tc.want)
			}
		})
	}
}
