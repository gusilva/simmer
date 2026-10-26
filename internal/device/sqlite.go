package device

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/house_arrest"

	"simmer/internal/logging"
)

// QuerySQLite executes a SQLite query against a database file in an app's
// private sandbox. The first returned row is the column header row.
//
// - iOS simulator: sqlite3 runs directly on the host against the simulator's local path.
// - iOS physical: the DB is pulled from the device via AFC/house_arrest, then queried locally.
// - Android: sqlite3 runs on-device via `adb shell run-as <pkg> sqlite3 …`.
func QuerySQLite(ctx context.Context, logger *logging.Logger, dev Device, packageID, dbPath, query string) ([][]string, error) {
	switch dev.Platform {
	case PlatformIOS:
		if dev.Kind == KindPhysical {
			return querySQLiteIOSPhysical(ctx, logger, dev, packageID, dbPath, query)
		}
		return querySQLiteLocal(ctx, logger, dbPath, query)
	case PlatformAndroid:
		return querySQLiteAndroid(ctx, logger, dev, packageID, dbPath, query)
	default:
		return nil, fmt.Errorf("sqlite: unsupported platform %s", dev.Platform)
	}
}

func querySQLiteLocal(ctx context.Context, logger *logging.Logger, dbPath, query string) ([][]string, error) {
	args := []string{"-csv", "-header", dbPath, query}
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "sqlite3", args...)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	logger.LogExec("sqlite3", args, string(out), err)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	return parseSQLiteCSV(out)
}

// querySQLiteIOSPhysical pulls the SQLite file from a physical iOS device to a
// temp file via AFC / house_arrest and then queries it using the local sqlite3.
func querySQLiteIOSPhysical(ctx context.Context, logger *logging.Logger, dev Device, packageID, dbPath, query string) ([][]string, error) {
	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		return nil, err
	}

	var client *afc.Client
	if packageID != "" {
		client, err = house_arrest.New(entry, packageID)
		logger.LogExec("go-ios", []string{"house_arrest", packageID}, "", err)
	} else {
		client, err = afc.New(entry)
		logger.LogExec("go-ios", []string{"afc", "connect"}, "", err)
	}
	if err != nil {
		return nil, fmt.Errorf("afc connect: %w", err)
	}
	defer client.Close()

	// Use a temp directory so WAL/SHM sidecar files land beside the main DB.
	// sqlite3 requires all three files to be co-located when the database is in
	// WAL mode; pulling only the main file causes "exit status" failures.
	tmpDir, err := os.MkdirTemp("", "simmer-sqlite-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDB := filepath.Join(tmpDir, filepath.Base(dbPath))

	pullErr := client.PullSingleFile(dbPath, tmpDB)
	logger.LogExec("go-ios", []string{"afc", "PullSingleFile", dbPath}, "", pullErr)
	if pullErr != nil {
		return nil, fmt.Errorf("pull %s: %w", dbPath, pullErr)
	}

	// Pull WAL and SHM sidecar files if they exist; ignore errors (they may not).
	for _, suffix := range []string{"-wal", "-shm"} {
		src := dbPath + suffix
		dst := tmpDB + suffix
		if err := client.PullSingleFile(src, dst); err != nil {
			logger.LogExec("go-ios", []string{"afc", "PullSingleFile", src}, "", err)
			os.Remove(dst)
		} else {
			logger.LogExec("go-ios", []string{"afc", "PullSingleFile", src}, "", nil)
		}
	}

	return querySQLiteLocal(ctx, logger, tmpDB, query)
}

// androidSQLiteOnDevice remembers, per adb serial, whether the device has a
// usable sqlite3 binary. Android system images stopped shipping one around
// Android 9, and the dbviewer issues many queries per session — probing once
// is enough.
var androidSQLiteOnDevice sync.Map // map[string]bool

func querySQLiteAndroid(ctx context.Context, logger *logging.Logger, dev Device, packageID, dbPath, query string) ([][]string, error) {
	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		return nil, fmt.Errorf("find serial: %w", err)
	}

	if onDevice, known := androidSQLiteOnDevice.Load(serial); known && !onDevice.(bool) {
		return queryAndroidViaCopy(ctx, logger, serial, dbPath, query)
	}

	rows, err := queryAndroidOnDevice(ctx, logger, serial, dbPath, query)
	if err == nil {
		androidSQLiteOnDevice.Store(serial, true)
		return rows, nil
	}
	if !isMissingSQLite3(err) {
		return nil, err
	}

	// No sqlite3 on the device: copy the database to this machine and use the
	// host's, the same approach physical iOS already takes.
	androidSQLiteOnDevice.Store(serial, false)
	logger.LogError("sqlite3 unavailable on "+serial+", querying a local copy", err)

	return queryAndroidViaCopy(ctx, logger, serial, dbPath, query)
}

// queryAndroidOnDevice runs the query with the device's own sqlite3 binary,
// as the app that owns the path when that path is private.
func queryAndroidOnDevice(ctx context.Context, logger *logging.Logger, serial, dbPath, query string) ([][]string, error) {
	shellCmd := androidRunAsForPath(dbPath) +
		"sqlite3 -csv -header " + sqSingleQuote(dbPath) + " " + sqSingleQuote(query)

	args := []string{"-s", serial, "shell", shellCmd}
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "adb", args...)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	logger.LogExec("adb", args, string(out), err)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("sqlite3: %w", err)
	}

	// adb shell reports the remote command's failure in its output, not in
	// its exit status, so a missing binary arrives as a successful run whose
	// text is a shell error.
	if msg := strings.TrimSpace(string(out)); isMissingSQLite3Text(msg) {
		return nil, fmt.Errorf("%s", msg)
	}

	return parseSQLiteCSV(out)
}

// queryAndroidViaCopy streams the database (and its WAL/SHM sidecars) off the
// device into a temp directory and queries it with the host's sqlite3.
//
// The copy is a snapshot: a write statement changes only the local file and
// never reaches the device. Each query takes a fresh copy, so reads are
// current.
func queryAndroidViaCopy(ctx context.Context, logger *logging.Logger, serial, dbPath, query string) ([][]string, error) {
	// A temp directory, not a temp file, so the sidecars can sit beside the
	// database: sqlite3 needs all three co-located to read a WAL-mode file.
	tmpDir, err := os.MkdirTemp("", "simmer-sqlite-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpDB := filepath.Join(tmpDir, filepath.Base(dbPath))
	if err := copyDeviceFile(ctx, logger, serial, dbPath, tmpDB); err != nil {
		return nil, err
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		if err := copyDeviceFile(ctx, logger, serial, dbPath+suffix, tmpDB+suffix); err != nil {
			// Sidecars are optional; a database not in WAL mode has none.
			_ = os.Remove(tmpDB + suffix)
		}
	}

	return querySQLiteLocal(ctx, logger, tmpDB, query)
}

// copyDeviceFile streams one file off the device into dst, reading it as the
// owning app when the path calls for it. exec-out is used because it does not
// translate the bytes in transit.
func copyDeviceFile(ctx context.Context, logger *logging.Logger, serial, src, dst string) error {
	shellCmd := androidRunAsForPath(src) + "cat " + sqSingleQuote(src)

	args := []string{"-s", serial, "exec-out", shellCmd}
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, "adb", args...)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	logger.LogExec("adb exec-out", args, fmt.Sprintf("%d bytes", len(out)), err)
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("read %s: %s", src, msg)
		}
		return fmt.Errorf("read %s: %w", src, err)
	}
	if len(out) == 0 {
		return fmt.Errorf("read %s: empty", src)
	}
	// A refused `cat` reports on stdout on some devices, so the refusal
	// would otherwise be written into the copy and reach sqlite3 as "file
	// is not a database".
	if msg := string(out); len(msg) < 512 && (strings.Contains(msg, "Permission denied") ||
		strings.Contains(msg, "No such file") ||
		strings.Contains(msg, "run-as:")) {
		return fmt.Errorf("read %s: %s", src, strings.TrimSpace(msg))
	}

	if err := os.WriteFile(dst, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

// isMissingSQLite3 reports whether err is the device telling us it has no
// usable sqlite3 binary, rather than a problem with the query or the file.
func isMissingSQLite3(err error) bool {
	return err != nil && isMissingSQLite3Text(err.Error())
}

func isMissingSQLite3Text(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "sqlite3") {
		return false
	}
	return strings.Contains(lower, "not found") ||
		strings.Contains(lower, "exec failed") ||
		strings.Contains(lower, "no such file") ||
		strings.Contains(lower, "permission denied") ||
		strings.Contains(lower, "inaccessible or not found")
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
func ListSQLiteObjects(ctx context.Context, logger *logging.Logger, dev Device, packageID, dbPath, query string) (SQLiteObjects, error) {
	rows, err := QuerySQLite(ctx, logger, dev, packageID, dbPath, query)
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
func SQLiteVersion(ctx context.Context, logger *logging.Logger, dev Device, packageID string) (string, error) {
	switch dev.Platform {
	case PlatformIOS:
		return sqliteVersionIOS(ctx, logger)
	case PlatformAndroid:
		return sqliteVersionAndroid(ctx, logger, dev, packageID)
	default:
		return "", fmt.Errorf("sqlite: unsupported platform %s", dev.Platform)
	}
}

func sqliteVersionIOS(ctx context.Context, logger *logging.Logger) (string, error) {
	args := []string{"--version"}
	out, err := exec.CommandContext(ctx, "sqlite3", args...).Output()
	logger.LogExec("sqlite3", args, string(out), err)
	if err != nil {
		return "", fmt.Errorf("sqlite3 --version: %w", err)
	}

	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("sqlite3: empty version output")
	}

	return parts[0], nil
}

func sqliteVersionAndroid(ctx context.Context, logger *logging.Logger, dev Device, packageID string) (string, error) {
	serial, err := resolveAndroidSerial(ctx, dev.ID)
	if err != nil {
		return "", fmt.Errorf("find serial: %w", err)
	}

	var shellCmd string
	if packageID == "" {
		shellCmd = "sqlite3 --version"
	} else {
		shellCmd = "run-as " + sqSingleQuote(packageID) + " sqlite3 --version"
	}

	args := []string{"-s", serial, "shell", shellCmd}
	out, err := exec.CommandContext(ctx, "adb", args...).Output()
	logger.LogExec("adb", args, string(out), err)

	text := strings.TrimSpace(string(out))
	parts := strings.Fields(text)

	// A device with no sqlite3 binary is served by the host's instead (see
	// queryAndroidViaCopy), so report the engine that actually answers the
	// queries rather than an error.
	if err != nil || len(parts) == 0 || isMissingSQLite3Text(text) {
		return sqliteVersionIOS(ctx, logger)
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
func QueryTableColumns(ctx context.Context, logger *logging.Logger, dev Device, packageID, dbPath, tableName string) ([]ColumnInfo, error) {
	infoRows, err := QuerySQLite(ctx, logger, dev, packageID, dbPath,
		"PRAGMA table_info("+sqIdentifier(tableName)+");")
	if err != nil {
		return nil, fmt.Errorf("table_info %s: %w", tableName, err)
	}

	fkRows, _ := QuerySQLite(ctx, logger, dev, packageID, dbPath,
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
