package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultTablesQuery is the SQL used to list schema objects when none is configured.
// The query must return at least a column named "name". An optional "type" column
// classifies objects (table/view/index/trigger); when absent all rows are shown
// as tables. An optional "custom_table_name" column overrides the display label
// while "name" remains the SQL identifier used for queries.
const DefaultTablesQuery = "SELECT type, name FROM sqlite_master" +
	" WHERE type IN ('table','view','index','trigger') AND name NOT LIKE 'sqlite_%'" +
	" ORDER BY type, name;"

// Config holds user-configurable settings persisted to ~/.config/simmer/config.toml.
type Config struct {
	ScriptPath  string
	TablesQuery string
}

// Default returns factory settings.
func Default() Config {
	return Config{
		ScriptPath:  "./",
		TablesQuery: DefaultTablesQuery,
	}
}

// EffectiveTablesQuery returns TablesQuery, falling back to DefaultTablesQuery
// when the stored value is empty.
func (c Config) EffectiveTablesQuery() string {
	if q := strings.TrimSpace(c.TablesQuery); q != "" {
		return q
	}
	return DefaultTablesQuery
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "simmer", "config.toml"), nil
}

// Load reads config from ~/.config/simmer/config.toml.
// A missing file is not an error — Default() is returned instead.
func Load() (Config, error) {
	cfg := Default()
	path, err := configPath()
	if err != nil {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = unquoteTOML(v)
		switch k {
		case "script_path":
			cfg.ScriptPath = v
		case "tables_query":
			cfg.TablesQuery = v
		}
	}
	return cfg, nil
}

// Save writes cfg to ~/.config/simmer/config.toml, creating parent dirs as needed.
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	content := "# simmer configuration\n\n" +
		"script_path = " + quoteTOML(cfg.ScriptPath) + "\n" +
		"tables_query = " + quoteTOML(cfg.TablesQuery) + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// quoteTOML wraps s in a TOML basic string, escaping special characters.
func quoteTOML(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}

// unquoteTOML reverses quoteTOML for a basic TOML string literal.
func unquoteTOML(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	s = s[1 : len(s)-1]
	// Process escape sequences left-to-right to avoid double-replacement.
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i+1])
			}
			i += 2
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}
