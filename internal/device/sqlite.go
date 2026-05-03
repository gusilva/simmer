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

// sqSingleQuote wraps s in single quotes, escaping embedded single quotes via '\”
func sqSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
