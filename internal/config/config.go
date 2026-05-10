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

// DBConfig holds settings that are stored per database.
type DBConfig struct {
	ScriptPath  string
	TablesQuery string
}

// EffectiveTablesQuery returns TablesQuery, falling back to DefaultTablesQuery
// when the stored value is empty.
func (d DBConfig) EffectiveTablesQuery() string {
	if q := strings.TrimSpace(d.TablesQuery); q != "" {
		return q
	}
	return DefaultTablesQuery
}

// Config holds user-configurable settings persisted to ~/.config/simmer/config.toml.
// Top-level fields are global defaults; Databases holds per-db overrides keyed by
// database display name.
type Config struct {
	ScriptPath  string // global default
	TablesQuery string // global default
	Databases   map[string]DBConfig
}

// Default returns factory settings.
func Default() Config {
	return Config{
		ScriptPath:  "./",
		TablesQuery: DefaultTablesQuery,
	}
}

// EffectiveTablesQuery returns the global TablesQuery, falling back to DefaultTablesQuery.
func (c Config) EffectiveTablesQuery() string {
	if q := strings.TrimSpace(c.TablesQuery); q != "" {
		return q
	}
	return DefaultTablesQuery
}

// ForDB returns the effective DBConfig for a named database.
// Per-db values override the global defaults; empty per-db fields inherit the global value.
// When dbName is empty the global values are returned directly.
func (c Config) ForDB(dbName string) DBConfig {
	base := DBConfig{
		ScriptPath:  c.ScriptPath,
		TablesQuery: c.TablesQuery,
	}
	if dbName == "" {
		return base
	}
	override, ok := c.Databases[dbName]
	if !ok {
		return base
	}
	if override.ScriptPath != "" {
		base.ScriptPath = override.ScriptPath
	}
	if override.TablesQuery != "" {
		base.TablesQuery = override.TablesQuery
	}
	return base
}

// WithDB returns a copy of c with the named database's config replaced.
func (c Config) WithDB(dbName string, dbcfg DBConfig) Config {
	if c.Databases == nil {
		c.Databases = make(map[string]DBConfig)
	} else {
		// Copy map so we don't mutate the original.
		m := make(map[string]DBConfig, len(c.Databases))
		for k, v := range c.Databases {
			m[k] = v
		}
		c.Databases = m
	}
	c.Databases[dbName] = dbcfg
	return c
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

	var currentDB string // "" = global section
	inDBSection := false

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header?
		if strings.HasPrefix(line, "[") {
			if dbName, ok := parseDBSectionHeader(line); ok {
				currentDB = dbName
				inDBSection = true
				if cfg.Databases == nil {
					cfg.Databases = make(map[string]DBConfig)
				}
				if _, exists := cfg.Databases[dbName]; !exists {
					cfg.Databases[dbName] = DBConfig{}
				}
			} else {
				// Unknown section — stop tracking any db section.
				inDBSection = false
				currentDB = ""
			}
			continue
		}

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = unquoteTOML(strings.TrimSpace(v))

		if inDBSection {
			dbcfg := cfg.Databases[currentDB]
			switch k {
			case "script_path":
				dbcfg.ScriptPath = v
			case "tables_query":
				dbcfg.TablesQuery = v
			}
			cfg.Databases[currentDB] = dbcfg
		} else {
			switch k {
			case "script_path":
				cfg.ScriptPath = v
			case "tables_query":
				cfg.TablesQuery = v
			}
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

	var sb strings.Builder
	sb.WriteString("# simmer configuration\n\n")
	sb.WriteString("script_path = " + quoteTOML(cfg.ScriptPath) + "\n")
	sb.WriteString("tables_query = " + quoteTOML(cfg.TablesQuery) + "\n")

	// Write per-db sections in deterministic order (sorted by name).
	if len(cfg.Databases) > 0 {
		names := make([]string, 0, len(cfg.Databases))
		for name := range cfg.Databases {
			names = append(names, name)
		}
		// Simple sort without importing sort package — insertion sort is fine for small N.
		for i := 1; i < len(names); i++ {
			for j := i; j > 0 && names[j] < names[j-1]; j-- {
				names[j], names[j-1] = names[j-1], names[j]
			}
		}
		for _, name := range names {
			dbcfg := cfg.Databases[name]
			sb.WriteString("\n[db." + quoteTOML(name) + "]\n")
			sb.WriteString("script_path = " + quoteTOML(dbcfg.ScriptPath) + "\n")
			sb.WriteString("tables_query = " + quoteTOML(dbcfg.TablesQuery) + "\n")
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// parseDBSectionHeader checks if line is a [db."<name>"] or [db.<name>] header.
func parseDBSectionHeader(line string) (dbName string, ok bool) {
	if !strings.HasPrefix(line, "[db.") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	inner := line[4 : len(line)-1] // content between "[db." and "]"
	if strings.HasPrefix(inner, `"`) {
		return unquoteTOML(inner), true
	}
	return inner, true
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
