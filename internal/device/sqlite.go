package device

import (
	"context"
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
	cmd := exec.CommandContext(ctx, "sqlite3", "-separator", "|", "-header", dbPath, query)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	return parseSQLiteOutput(string(out)), nil
}

func querySQLiteAndroid(ctx context.Context, dev Device, packageID, dbPath, query string) ([][]string, error) {
	serial, err := findAndroidSerial(ctx, dev.ID)
	if err != nil {
		return nil, fmt.Errorf("find serial: %w", err)
	}

	// When packageID is empty we are browsing as root; run sqlite3 directly.
	var shellCmd string
	if packageID == "" {
		shellCmd = "sqlite3 -separator '|' -header " +
			sqSingleQuote(dbPath) + " " + sqSingleQuote(query)
	} else {
		shellCmd = "run-as " + sqSingleQuote(packageID) +
			" sqlite3 -separator '|' -header " +
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
	return parseSQLiteOutput(string(out)), nil
}

func parseSQLiteOutput(raw string) [][]string {
	data := strings.TrimRight(raw, "\n\r")
	if data == "" {
		return nil
	}
	var rows [][]string
	for line := range strings.SplitSeq(data, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		rows = append(rows, strings.Split(line, "|"))
	}
	return rows
}

// SQLiteObjects holds the names of schema objects in a SQLite database grouped
// by type.
type SQLiteObjects struct {
	Tables  []string
	Views   []string
	Indexes []string
}

// ListSQLiteObjects queries sqlite_master and returns tables, views, and
// indexes for the given database file, excluding SQLite-internal objects.
func ListSQLiteObjects(ctx context.Context, dev Device, packageID, dbPath string) (SQLiteObjects, error) {
	const q = "SELECT type, name FROM sqlite_master" +
		" WHERE type IN ('table','view','index') AND name NOT LIKE 'sqlite_%'" +
		" ORDER BY type, name;"
	rows, err := QuerySQLite(ctx, dev, packageID, dbPath, q)
	if err != nil {
		return SQLiteObjects{}, err
	}

	var out SQLiteObjects
	for _, row := range rows[1:] { // row 0 is the header from -header flag
		if len(row) < 2 {
			continue
		}
		switch row[0] {
		case "table":
			out.Tables = append(out.Tables, row[1])
		case "view":
			out.Views = append(out.Views, row[1])
		case "index":
			out.Indexes = append(out.Indexes, row[1])
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
