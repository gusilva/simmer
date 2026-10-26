package session

import (
	"context"
	"testing"

	"simmer/internal/device"
)

func TestDatabasesFindsSQLiteFilesAtAnyDepth(t *testing.T) {
	sandbox := device.FileNode{
		Name:  "com.acme.fieldkit",
		Path:  "/data/data/com.acme.fieldkit",
		IsDir: true,
		Children: []device.FileNode{
			{Name: "cache.sqlite", Path: "/data/data/com.acme.fieldkit/cache.sqlite", Size: 1200},
			{Name: "sync.log", Path: "/data/data/com.acme.fieldkit/sync.log", Size: 400},
			{
				Name:  "databases",
				Path:  "/data/data/com.acme.fieldkit/databases",
				IsDir: true,
				Children: []device.FileNode{
					{Name: "rooms.db", Path: "/data/data/com.acme.fieldkit/databases/rooms.db", Size: 90},
					{Name: "rooms.db-wal", Path: "/data/data/com.acme.fieldkit/databases/rooms.db-wal"},
					{Name: "old.sqlite3", Path: "/data/data/com.acme.fieldkit/databases/old.sqlite3"},
				},
			},
		},
	}

	s := androidSession(&fakeManager{kind: device.KindVirtual, appTree: sandbox})

	dbs, err := s.Databases(context.Background(), androidDevice(device.KindVirtual), "com.acme.fieldkit")
	if err != nil {
		t.Fatalf("Databases: %v", err)
	}

	var names []string
	for _, d := range dbs {
		names = append(names, d.Name)
	}
	want := []string{"cache.sqlite", "rooms.db", "old.sqlite3"}
	if len(names) != len(want) {
		t.Fatalf("found %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("found[%d] = %q, want %q", i, names[i], w)
		}
	}

	if dbs[0].Path != "/data/data/com.acme.fieldkit/cache.sqlite" || dbs[0].Size != 1200 {
		t.Errorf("unexpected first database: %+v", dbs[0])
	}
}

func TestDatabasesAtFindsSharedStorageDatabases(t *testing.T) {
	// A database exported to a Downloads folder belongs to no app, so no
	// bundle id can reach it.
	download := device.FileNode{
		Name:  "Download",
		Path:  "/storage/emulated/0/Download",
		IsDir: true,
		Children: []device.FileNode{
			{Name: "export.db", Path: "/storage/emulated/0/Download/export.db", Size: 4096},
			{Name: "notes.txt", Path: "/storage/emulated/0/Download/notes.txt"},
		},
	}
	s := androidSession(&fakeManager{kind: device.KindVirtual, pathTree: download})

	dbs, err := s.DatabasesAt(context.Background(), androidDevice(device.KindVirtual), download.Path)
	if err != nil {
		t.Fatalf("DatabasesAt: %v", err)
	}
	if len(dbs) != 1 || dbs[0].Name != "export.db" {
		t.Fatalf("got %+v, want just export.db", dbs)
	}
	if dbs[0].Path != "/storage/emulated/0/Download/export.db" {
		t.Errorf("path = %q", dbs[0].Path)
	}
}

func TestQueryWithoutPathFails(t *testing.T) {
	s := androidSession(&fakeManager{kind: device.KindVirtual})

	if _, err := s.Query(context.Background(), DBTarget{Device: androidDevice(device.KindVirtual)}, "select 1"); err == nil {
		t.Fatal("expected an error when no database path is given")
	}
}

func TestSearchFilesReachesPastTheLoadedTree(t *testing.T) {
	f := &fakeManager{kind: device.KindVirtual}
	f.platform = device.PlatformAndroid
	s := New(device.NewCoordinator(f, &fakeSearchManager{
		results: map[string][]device.FileMatch{
			"/storage/emulated/0": {{Name: "rooms.db", Path: "/storage/emulated/0/Download/rooms.db"}},
		},
	}), nil, testConfig())

	got, err := s.SearchFiles(context.Background(), androidDevice(device.KindVirtual), []string{"/storage/emulated/0"}, "rooms")
	if err != nil {
		t.Fatalf("SearchFiles: %v", err)
	}
	if len(got) != 1 || got[0].Name != "rooms.db" {
		t.Errorf("got %+v, want the device's match", got)
	}
}

// fakeSearchManager answers searches without a device.
type fakeSearchManager struct {
	results map[string][]device.FileMatch
}

func (f *fakeSearchManager) ListDevices(context.Context) ([]device.Device, error) { return nil, nil }
func (f *fakeSearchManager) Platform() device.Platform                            { return device.PlatformAndroid }
func (f *fakeSearchManager) Kind() device.DeviceKind                              { return device.KindVirtual }
func (f *fakeSearchManager) SearchFiles(_ context.Context, _ device.Device, root, _ string, _ int) ([]device.FileMatch, error) {
	return f.results[root], nil
}
