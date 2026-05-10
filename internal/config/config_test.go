package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- DBConfig ----------

func TestDBConfig_EffectiveTablesQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"empty uses default", "", DefaultTablesQuery},
		{"whitespace uses default", "   ", DefaultTablesQuery},
		{"custom query returned as-is", "SELECT name FROM sqlite_master;", "SELECT name FROM sqlite_master;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := DBConfig{TablesQuery: tc.query}
			if got := d.EffectiveTablesQuery(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------- Config.ForDB ----------

func TestConfig_ForDB_NoEntry(t *testing.T) {
	cfg := Config{ScriptPath: "/global", TablesQuery: "SELECT 1;"}
	got := cfg.ForDB("missing.db")
	if got.ScriptPath != "/global" {
		t.Errorf("ScriptPath: got %q, want /global", got.ScriptPath)
	}
	if got.TablesQuery != "SELECT 1;" {
		t.Errorf("TablesQuery: got %q, want SELECT 1;", got.TablesQuery)
	}
}

func TestConfig_ForDB_EmptyName(t *testing.T) {
	cfg := Config{ScriptPath: "/global", TablesQuery: "SELECT 1;"}
	got := cfg.ForDB("")
	if got.ScriptPath != "/global" {
		t.Errorf("ScriptPath: got %q, want /global", got.ScriptPath)
	}
}

func TestConfig_ForDB_FullOverride(t *testing.T) {
	cfg := Config{
		ScriptPath:  "/global",
		TablesQuery: "SELECT 1;",
		Databases: map[string]DBConfig{
			"app.db": {ScriptPath: "/app", TablesQuery: "SELECT 2;"},
		},
	}
	got := cfg.ForDB("app.db")
	if got.ScriptPath != "/app" {
		t.Errorf("ScriptPath: got %q, want /app", got.ScriptPath)
	}
	if got.TablesQuery != "SELECT 2;" {
		t.Errorf("TablesQuery: got %q, want SELECT 2;", got.TablesQuery)
	}
}

func TestConfig_ForDB_PartialOverride_ScriptPathOnly(t *testing.T) {
	cfg := Config{
		ScriptPath:  "/global",
		TablesQuery: "SELECT 1;",
		Databases: map[string]DBConfig{
			"app.db": {ScriptPath: "/app"},
		},
	}
	got := cfg.ForDB("app.db")
	if got.ScriptPath != "/app" {
		t.Errorf("ScriptPath: got %q, want /app", got.ScriptPath)
	}
	// Empty TablesQuery in override → falls back to global
	if got.TablesQuery != "SELECT 1;" {
		t.Errorf("TablesQuery: got %q, want SELECT 1;", got.TablesQuery)
	}
}

func TestConfig_ForDB_PartialOverride_QueryOnly(t *testing.T) {
	cfg := Config{
		ScriptPath:  "/global",
		TablesQuery: "SELECT 1;",
		Databases: map[string]DBConfig{
			"app.db": {TablesQuery: "SELECT 2;"},
		},
	}
	got := cfg.ForDB("app.db")
	if got.ScriptPath != "/global" {
		t.Errorf("ScriptPath: got %q, want /global", got.ScriptPath)
	}
	if got.TablesQuery != "SELECT 2;" {
		t.Errorf("TablesQuery: got %q, want SELECT 2;", got.TablesQuery)
	}
}

// ---------- Config.WithDB ----------

func TestConfig_WithDB_AddsEntry(t *testing.T) {
	cfg := Config{ScriptPath: "/global"}
	updated := cfg.WithDB("new.db", DBConfig{ScriptPath: "/new"})
	if updated.Databases["new.db"].ScriptPath != "/new" {
		t.Errorf("expected /new, got %q", updated.Databases["new.db"].ScriptPath)
	}
}

func TestConfig_WithDB_UpdatesExisting(t *testing.T) {
	cfg := Config{
		Databases: map[string]DBConfig{
			"app.db": {ScriptPath: "/old"},
		},
	}
	updated := cfg.WithDB("app.db", DBConfig{ScriptPath: "/new"})
	if updated.Databases["app.db"].ScriptPath != "/new" {
		t.Errorf("expected /new, got %q", updated.Databases["app.db"].ScriptPath)
	}
}

func TestConfig_WithDB_DoesNotMutateOriginal(t *testing.T) {
	cfg := Config{
		Databases: map[string]DBConfig{
			"app.db": {ScriptPath: "/original"},
		},
	}
	_ = cfg.WithDB("app.db", DBConfig{ScriptPath: "/mutated"})
	if cfg.Databases["app.db"].ScriptPath != "/original" {
		t.Errorf("original config was mutated")
	}
}

func TestConfig_WithDB_NilMap(t *testing.T) {
	cfg := Config{} // Databases is nil
	updated := cfg.WithDB("app.db", DBConfig{ScriptPath: "/path"})
	if updated.Databases["app.db"].ScriptPath != "/path" {
		t.Errorf("expected /path, got %q", updated.Databases["app.db"].ScriptPath)
	}
}

// ---------- parseDBSectionHeader ----------

func TestParseDBSectionHeader(t *testing.T) {
	tests := []struct {
		line    string
		wantDB  string
		wantOK  bool
	}{
		{`[db."app.db"]`, "app.db", true},
		{`[db."my database"]`, "my database", true},
		{`[db.simple]`, "simple", true},
		{`[other]`, "", false},
		{`[databases."x"]`, "", false},
		{`not a section`, "", false},
		{`[db.]`, "", true}, // edge: empty name after dot
	}
	for _, tc := range tests {
		t.Run(tc.line, func(t *testing.T) {
			gotDB, gotOK := parseDBSectionHeader(tc.line)
			if gotOK != tc.wantOK {
				t.Errorf("ok: got %v, want %v", gotOK, tc.wantOK)
			}
			if gotDB != tc.wantDB {
				t.Errorf("name: got %q, want %q", gotDB, tc.wantDB)
			}
		})
	}
}

// ---------- quoteTOML / unquoteTOML ----------

func TestQuoteUnquoteTOML_RoundTrip(t *testing.T) {
	inputs := []string{
		"",
		"simple",
		`path/with "quotes"`,
		"line\nbreak",
		`back\slash`,
		"tab\there",
		"/Users/gustavo/scripts/my db.sql",
	}
	for _, s := range inputs {
		t.Run(s, func(t *testing.T) {
			quoted := quoteTOML(s)
			got := unquoteTOML(quoted)
			if got != s {
				t.Errorf("round-trip failed: %q → %q → %q", s, quoted, got)
			}
		})
	}
}

func TestUnquoteTOML_NotQuoted(t *testing.T) {
	// Values without surrounding quotes are returned as-is.
	if got := unquoteTOML("bare"); got != "bare" {
		t.Errorf("expected bare, got %q", got)
	}
}

// ---------- Load / Save round-trip ----------

func writeConfigFile(t *testing.T, content string) (restore func()) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	dir := filepath.Join(home, ".config", "simmer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "config.toml")

	// Preserve existing file if present.
	existing, readErr := os.ReadFile(path)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return func() {
		if readErr == nil {
			_ = os.WriteFile(path, existing, 0o644)
		} else {
			_ = os.Remove(path)
		}
	}
}

func TestLoad_MissingFile(t *testing.T) {
	// Point to a path that cannot exist by temporarily renaming any real file.
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "simmer", "config.toml")
	_, err := os.Stat(path)
	if err == nil {
		t.Skip("config file exists; skipping missing-file test to avoid side effects")
	}
	cfg, err := Load()
	if err != nil {
		t.Errorf("expected no error for missing file, got %v", err)
	}
	if cfg.ScriptPath == "" {
		t.Errorf("expected non-empty default ScriptPath")
	}
}

func TestLoad_GlobalOnly(t *testing.T) {
	restore := writeConfigFile(t, `
# comment
script_path = "/my/scripts"
tables_query = "SELECT name FROM sqlite_master;"
`)
	defer restore()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ScriptPath != "/my/scripts" {
		t.Errorf("ScriptPath: got %q, want /my/scripts", cfg.ScriptPath)
	}
	if cfg.TablesQuery != "SELECT name FROM sqlite_master;" {
		t.Errorf("TablesQuery: got %q", cfg.TablesQuery)
	}
	if len(cfg.Databases) != 0 {
		t.Errorf("expected no per-db entries, got %d", len(cfg.Databases))
	}
}

func TestLoad_WithDBSection(t *testing.T) {
	restore := writeConfigFile(t, `
script_path = "/global"
tables_query = ""

[db."app.db"]
script_path = "/app/scripts"
tables_query = "SELECT name FROM sqlite_master WHERE type='table';"
`)
	defer restore()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ScriptPath != "/global" {
		t.Errorf("global ScriptPath: got %q", cfg.ScriptPath)
	}
	dbcfg, ok := cfg.Databases["app.db"]
	if !ok {
		t.Fatal("expected db entry for app.db")
	}
	if dbcfg.ScriptPath != "/app/scripts" {
		t.Errorf("db ScriptPath: got %q, want /app/scripts", dbcfg.ScriptPath)
	}
	if !strings.Contains(dbcfg.TablesQuery, "type='table'") {
		t.Errorf("db TablesQuery unexpected: %q", dbcfg.TablesQuery)
	}
}

func TestLoad_MultipleDBSections(t *testing.T) {
	restore := writeConfigFile(t, `
script_path = "/global"
tables_query = ""

[db."first.db"]
script_path = "/first"
tables_query = ""

[db."second.db"]
script_path = "/second"
tables_query = "SELECT 2;"
`)
	defer restore()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Databases) != 2 {
		t.Errorf("expected 2 db entries, got %d", len(cfg.Databases))
	}
	if cfg.Databases["first.db"].ScriptPath != "/first" {
		t.Errorf("first.db: got %q", cfg.Databases["first.db"].ScriptPath)
	}
	if cfg.Databases["second.db"].TablesQuery != "SELECT 2;" {
		t.Errorf("second.db query: got %q", cfg.Databases["second.db"].TablesQuery)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	restore := writeConfigFile(t, "")
	defer restore()

	original := Config{
		ScriptPath:  "/global/scripts",
		TablesQuery: "SELECT type, name FROM sqlite_master;",
		Databases: map[string]DBConfig{
			"app.db": {
				ScriptPath:  "/app/scripts",
				TablesQuery: "SELECT name FROM app_tables;",
			},
			"other.db": {
				ScriptPath:  "/other",
				TablesQuery: "",
			},
		},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.ScriptPath != original.ScriptPath {
		t.Errorf("ScriptPath: got %q, want %q", loaded.ScriptPath, original.ScriptPath)
	}
	if loaded.TablesQuery != original.TablesQuery {
		t.Errorf("TablesQuery: got %q, want %q", loaded.TablesQuery, original.TablesQuery)
	}
	if len(loaded.Databases) != 2 {
		t.Errorf("expected 2 db entries, got %d", len(loaded.Databases))
	}
	if loaded.Databases["app.db"].ScriptPath != "/app/scripts" {
		t.Errorf("app.db ScriptPath: got %q", loaded.Databases["app.db"].ScriptPath)
	}
	if loaded.Databases["app.db"].TablesQuery != "SELECT name FROM app_tables;" {
		t.Errorf("app.db TablesQuery: got %q", loaded.Databases["app.db"].TablesQuery)
	}
	if loaded.Databases["other.db"].ScriptPath != "/other" {
		t.Errorf("other.db ScriptPath: got %q", loaded.Databases["other.db"].ScriptPath)
	}
}

func TestSaveLoad_SpecialCharsInDBName(t *testing.T) {
	restore := writeConfigFile(t, "")
	defer restore()

	name := `my "special" db.sqlite`
	cfg := Config{
		ScriptPath: "/g",
		Databases: map[string]DBConfig{
			name: {ScriptPath: "/s"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Databases[name].ScriptPath != "/s" {
		t.Errorf("db with special name not preserved: got %+v", loaded.Databases)
	}
}
