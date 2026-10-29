package session

import (
	"context"
	"fmt"

	"simmer/internal/device"
)

// DBTarget identifies one SQLite database on a device: which device, whose
// sandbox it lives in, and where. PackageID is empty when browsing as root
// rather than through an app's private container. Name is the display name
// used to look up per-database configuration; it defaults to the file's base
// name.
type DBTarget struct {
	Device    device.Device
	PackageID string
	Path      string
	Name      string
}

// DatabaseFile is a SQLite database discovered inside an app's sandbox.
type DatabaseFile struct {
	Name string
	Path string
	Size int64
}

// Databases walks bundleID's own storage on dev and returns every SQLite
// database file in it. This is how a caller learns the paths it can query:
// a database path is not derivable from a bundle id alone.
func (s *Session) Databases(ctx context.Context, dev device.Device, bundleID string) ([]DatabaseFile, error) {
	return s.databasesIn(ctx, dev, device.AppStorage(bundleID))
}

// DatabasesAt walks an arbitrary directory on dev and returns every SQLite
// database file under it. Shared storage belongs to no app, so a path is the
// only way to name it; databases exported to a Downloads folder are reachable
// this way and through no bundle id.
func (s *Session) DatabasesAt(ctx context.Context, dev device.Device, path string) ([]DatabaseFile, error) {
	return s.databasesIn(ctx, dev, device.Directory(path))
}

func (s *Session) databasesIn(ctx context.Context, dev device.Device, t device.FileTarget) ([]DatabaseFile, error) {
	root, err := s.FileTree(ctx, dev, t, device.FullDepth)
	if err != nil {
		return nil, err
	}

	var out []DatabaseFile
	collectDatabases(root, &out)
	return out, nil
}

func collectDatabases(n device.FileNode, out *[]DatabaseFile) {
	if !n.IsDir && device.IsDatabaseFile(n.Name) {
		*out = append(*out, DatabaseFile{Name: n.Name, Path: n.Path, Size: n.Size})
	}
	for _, c := range n.Children {
		collectDatabases(c, out)
	}
}

// Query runs sql against the target database and returns the result rows,
// the first of which is the column header row.
func (s *Session) Query(ctx context.Context, t DBTarget, sql string) ([][]string, error) {
	if t.Path == "" {
		return nil, fmt.Errorf("no database path given")
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return device.QuerySQLite(ctx, s.logger, t.Device, t.PackageID, t.Path, sql)
}

// SchemaObjects lists the target database's tables, views, indexes and
// triggers using the tables query configured for that database, falling back
// to the global and then the built-in default.
func (s *Session) SchemaObjects(ctx context.Context, t DBTarget) (device.SQLiteObjects, error) {
	ctx, cancel := context.WithTimeout(ctx, schemaTimeout)
	defer cancel()
	q := s.Config().ForDB(t.Name).EffectiveTablesQuery()
	return device.ListSQLiteObjects(ctx, s.logger, t.Device, t.PackageID, t.Path, q)
}

// TableColumns describes one table's columns, including primary- and
// foreign-key marks.
func (s *Session) TableColumns(ctx context.Context, t DBTarget, table string) ([]device.ColumnInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, schemaTimeout)
	defer cancel()
	return device.QueryTableColumns(ctx, s.logger, t.Device, t.PackageID, t.Path, table)
}

// SQLiteVersion reports the SQLite engine version serving the target.
func (s *Session) SQLiteVersion(ctx context.Context, t DBTarget) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, appsTimeout)
	defer cancel()
	return device.SQLiteVersion(ctx, s.logger, t.Device, t.PackageID)
}

// SearchFiles finds files and directories by name under everything the
// target covers.
//
// A tree loads one directory at a time, so filtering what is on screen can
// only ever match what has already been expanded; this reaches the rest of
// the device.
func (s *Session) SearchFiles(ctx context.Context, dev device.Device, t device.FileTarget, query string) ([]device.FileMatch, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceTimeout(dev))
	defer cancel()
	return s.coord.SearchFiles(ctx, dev, t, query, device.DefaultSearchLimit)
}
