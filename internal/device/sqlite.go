package device

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strings"
)

// QuerySQLite executes a SQLite query against a database file in an app's
// private sandbox. The first returned row is the column header row.
//
// [TODO] Implement a buildin SQLite client in Go to avoid the overhead of spawning a process and
// parsing its output. This would also allow us to support more complex queries and
// handle edge cases more robustly (e.g. embedded newlines in query results).
//
// - iOS: sqlite3 runs directly on the host against the simulator's local path.
// - Android: sqlite3 runs on-device via `adb shell run-as <pkg> sqlite3 …`.
// The command is passed as a single shell string so the '|' separator is
// quoted and not interpreted as a pipe by the device shell.
func QuerySQLite(ctx context.Context, dev Device, packageID, dbPath, query string) ([][]string, error) {
	switch dev.Platform {
	case PlatformIOS:
		return querySQLiteIOS(ctx, dbPath, query)
	case PlatformAndroid:
		return querySQLiteAndroid(ctx, dev, packageID, dbPath, query)
	default:
		return nil, fmt.Errorf("sqlite: unsupported platform %s", dev.Platform)
	}
}

func querySQLiteIOS(ctx context.Context, dbPath, query string) ([][]string, error) {
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "sqlite3", "-csv", "-header", dbPath, query)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	return parseSQLiteCSV(out)
}

func querySQLiteAndroid(ctx context.Context, dev Device, packageID, dbPath, query string) ([][]string, error) {
	serial, err := findAndroidSerial(ctx, dev.ID)
	if err != nil {
		return nil, fmt.Errorf("find serial: %w", err)
	}

	// When packageID is empty we are browsing as root; run sqlite3 directly.
	var shellCmd string
	if packageID == "" {
		shellCmd = "sqlite3 -csv -header " +
			sqSingleQuote(dbPath) + " " + sqSingleQuote(query)
	} else {
		shellCmd = "run-as " + sqSingleQuote(packageID) +
			" sqlite3 -csv -header " +
			sqSingleQuote(dbPath) + " " + sqSingleQuote(query)
	}

	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", shellCmd)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	return parseSQLiteCSV(out)
}

func parseSQLiteCSV(data []byte) ([][]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(string(data)))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // allow variable column count
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows, nil
}

// SchemaObject represents one named object from the SQLite schema.
// DisplayName is the human-readable label (from an optional third query
// column); when empty the real Name is shown instead.
type SchemaObject struct {
	Name        string // actual identifier used in SQL
	DisplayName string // optional custom display label
}

// Label returns DisplayName if set, otherwise Name.
func (s SchemaObject) Label() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return s.Name
}

// SQLiteObjects holds schema objects in a SQLite database grouped by type.
type SQLiteObjects struct {
	Tables   []SchemaObject
	Views    []SchemaObject
	Indexes  []SchemaObject
	Triggers []SchemaObject
}

// ListSQLiteObjects queries the database using the provided SQL and returns
// schema objects grouped by type.
//
// Column detection is done by name from the header row (sqlite3 -header), so
// column order does not matter. Recognised column names (case-insensitive):
//
//	name              — required; SQL identifier used in queries
//	type              — optional; "table", "view", "index", or "trigger".
//	                    When absent every row is treated as type "table".
//	custom_table_name — optional; human-readable display label.
//	                    When absent or empty the name column is shown instead.
func ListSQLiteObjects(ctx context.Context, dev Device, packageID, dbPath, query string) (SQLiteObjects, error) {
	rows, err := QuerySQLite(ctx, dev, packageID, dbPath, query)
	if err != nil {
		return SQLiteObjects{}, err
	}
	if len(rows) == 0 {
		return SQLiteObjects{}, nil
	}

	// Locate columns by name from the header row.
	typeIdx, nameIdx, displayIdx := -1, -1, -1
	for i, col := range rows[0] {
		switch strings.ToLower(col) {
		case "type":
			typeIdx = i
		case "name":
			nameIdx = i
		case "custom_table_name":
			displayIdx = i
		}
	}
	if nameIdx == -1 {
		return SQLiteObjects{}, fmt.Errorf("tables query must return a column named 'name'")
	}

	var out SQLiteObjects
	for _, row := range rows[1:] {
		if nameIdx >= len(row) || row[nameIdx] == "" {
			continue
		}
		obj := SchemaObject{Name: row[nameIdx]}
		if displayIdx >= 0 && displayIdx < len(row) {
			obj.DisplayName = row[displayIdx]
		}

		objType := "table"
		if typeIdx >= 0 && typeIdx < len(row) {
			objType = row[typeIdx]
		}

		switch objType {
		case "table":
			out.Tables = append(out.Tables, obj)
		case "view":
			out.Views = append(out.Views, obj)
		case "index":
			out.Indexes = append(out.Indexes, obj)
		case "trigger":
			out.Triggers = append(out.Triggers, obj)
		}
	}

	return out, nil
}

// SQLiteVersion returns the SQLite version string from the engine on the given
// device. For iOS the local sqlite3 binary is queried; for Android the version
// is obtained via adb shell (with run-as when packageID is non-empty).
func SQLiteVersion(ctx context.Context, dev Device, packageID string) (string, error) {
	switch dev.Platform {
	case PlatformIOS:
		return sqliteVersionIOS(ctx)
	case PlatformAndroid:
		return sqliteVersionAndroid(ctx, dev, packageID)
	default:
		return "", fmt.Errorf("sqlite: unsupported platform %s", dev.Platform)
	}
}

func sqliteVersionIOS(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "sqlite3", "--version").Output()
	if err != nil {
		return "", fmt.Errorf("sqlite3 --version: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("sqlite3: empty version output")
	}

	return parts[0], nil
}

func sqliteVersionAndroid(ctx context.Context, dev Device, packageID string) (string, error) {
	serial, err := findAndroidSerial(ctx, dev.ID)
	if err != nil {
		return "", fmt.Errorf("find serial: %w", err)
	}

	var shellCmd string
	if packageID == "" {
		shellCmd = "sqlite3 --version"
	} else {
		shellCmd = "run-as " + sqSingleQuote(packageID) + " sqlite3 --version"
	}

	out, err := exec.CommandContext(ctx, "adb", "-s", serial, "shell", shellCmd).Output()
	if err != nil {
		return "", fmt.Errorf("adb sqlite3 --version: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("adb sqlite3: empty version output")
	}

	return parts[0], nil
}

// ColumnInfo describes one column in a SQLite table.
type ColumnInfo struct {
	Name string
	Type string
	IsPK bool
	IsFK bool
}

// QueryTableColumns returns the columns of tableName including primary-key and
// foreign-key flags. PK info comes from PRAGMA table_info; FK info from
// PRAGMA foreign_key_list.
func QueryTableColumns(ctx context.Context, dev Device, packageID, dbPath, tableName string) ([]ColumnInfo, error) {
	infoRows, err := QuerySQLite(ctx, dev, packageID, dbPath,
		"PRAGMA table_info("+sqIdentifier(tableName)+");")
	if err != nil {
		return nil, fmt.Errorf("table_info %s: %w", tableName, err)
	}

	fkRows, _ := QuerySQLite(ctx, dev, packageID, dbPath,
		"PRAGMA foreign_key_list("+sqIdentifier(tableName)+");")

	fkCols := make(map[string]bool)
	for i, row := range fkRows {
		if i == 0 {
			continue // header
		}

		if len(row) >= 4 {
			fkCols[row[3]] = true // "from" column
		}
	}

	var cols []ColumnInfo
	for i, row := range infoRows {
		if i == 0 {
			continue // header: cid|name|type|notnull|dflt_value|pk
		}

		if len(row) < 6 {
			continue
		}

		cols = append(cols, ColumnInfo{
			Name: row[1],
			Type: row[2],
			IsPK: row[5] != "0",
			IsFK: fkCols[row[1]],
		})
	}

	return cols, nil
}

// sqSingleQuote wraps s in single quotes, escaping embedded single quotes.
func sqSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// sqIdentifier wraps s in double quotes for use as a SQLite identifier.
func sqIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
