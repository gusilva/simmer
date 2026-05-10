## `internal/config/` — user configuration

Persistent key-value settings stored in `~/.config/simmer/config.toml`.
No external library — hand-rolled TOML basic-string codec.

---

## Types

```go
type Config struct {
    ScriptPath  string   // default: "./"
    TablesQuery string   // default: "" (empty = use DefaultTablesQuery)
}
```

`TablesQuery` uses an **empty-means-default** convention rather than storing
the default string. This lets the user clear the field in Settings and instantly
revert to the built-in query without having to know what it is.

```go
func (c Config) EffectiveTablesQuery() string {
    if q := strings.TrimSpace(c.TablesQuery); q != "" {
        return q
    }
    return DefaultTablesQuery
}
```

All callers use `EffectiveTablesQuery()` rather than `TablesQuery` directly.

---

## `DefaultTablesQuery`

```sql
SELECT type, name FROM sqlite_master
WHERE type IN ('table','view','index','trigger')
  AND name NOT LIKE 'sqlite_%'
ORDER BY type, name;
```

Returns the `type` and `name` columns that `device.ListSQLiteObjects` expects.
Filters out SQLite internal objects (`sqlite_stat1`, etc.).

---

## Load / Save

```
~/.config/simmer/config.toml

# simmer configuration

script_path = "./"
tables_query = "SELECT type, name FROM sqlite_master…"
```

`Load` — reads the file; missing file is not an error (returns `Default()`).
Parses line by line: skip blanks and `#` comments, split on first `=`, unquote value.

`Save` — writes the full file atomically via `os.WriteFile` (not append).
Creates parent dirs if missing. All values are TOML basic-string quoted.

### TOML codec

`quoteTOML` / `unquoteTOML` handle only what the config actually stores:
`"`, `\`, `\n`, `\r`, `\t`. No multi-line strings, no arrays, no tables.
This avoids a dependency on a TOML library for a two-field config file.

`unquoteTOML` processes escapes left-to-right in a single pass to avoid
double-replacement bugs (e.g., `\\n` must not become `\n` then `↵`).

---

## Why this way?

**No external library:** The config format is intentionally minimal. Adding
`github.com/BurntSushi/toml` for two string keys would be over-engineering.
The hand-rolled codec is 40 lines and covers exactly what is needed.

**`EffectiveTablesQuery` at the call site:** The fallback logic lives in one
method, not scattered across callers. When the default query changes, one edit
fixes all consumers.

**`Load` called on each Settings open / SetFile:** Config is cheap to read
(a few hundred bytes from disk). Re-reading on demand means the TUI always
reflects what is on disk, even if the file was edited externally between
invocations.
