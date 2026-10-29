package device

import (
	"context"
	"os"
	gopath "path"
	"strings"
	"testing"
)

// fakeSearcher is a Manager that answers searches from a canned path list.
type fakeSearcher struct {
	platform Platform
	kind     DeviceKind
	byRoot   map[string][]string
	calls    []string
	err      error
}

func (f *fakeSearcher) ListDevices(context.Context) ([]Device, error) { return nil, nil }
func (f *fakeSearcher) Platform() Platform                            { return f.platform }
func (f *fakeSearcher) Kind() DeviceKind                              { return f.kind }

func (f *fakeSearcher) FileSystemFor(FileTarget, Depth) FileSystem { return nil }

func (f *fakeSearcher) StorageRoots(_ context.Context, _ Device, t FileTarget) ([]string, error) {
	if t.BundleID() != "" {
		return []string{"/data/data/" + t.BundleID(), "/storage/emulated/0/Android/data/" + t.BundleID()}, nil
	}
	return []string{t.Path()}, nil
}

func (f *fakeSearcher) SearchFiles(_ context.Context, _ Device, root, query string, limit int) ([]FileMatch, error) {
	f.calls = append(f.calls, root)
	if f.err != nil {
		return nil, f.err
	}

	var out []FileMatch
	for _, p := range f.byRoot[root] {
		if !matchesQuery(gopath.Base(p), p, query) || len(out) >= limit {
			continue
		}
		out = append(out, FileMatch{Name: gopath.Base(p), Path: p})
	}
	return out, nil
}

func searchCoordinator(f *fakeSearcher) (*Coordinator, Device) {
	f.platform, f.kind = PlatformAndroid, KindVirtual
	return NewCoordinator(f), Device{ID: "X", Platform: PlatformAndroid, Kind: KindVirtual}
}

func TestSearchFilesMergesRootsAndDeduplicates(t *testing.T) {
	// An app's storage spans two roots on Android, and a path reachable
	// from both must still be listed once.
	shared := "/data/data/com.acme.app/databases/rooms.db"
	f := &fakeSearcher{byRoot: map[string][]string{
		"/data/data/com.acme.app":                       {shared, "/data/data/com.acme.app/cache/other.txt"},
		"/storage/emulated/0/Android/data/com.acme.app": {shared, "/storage/emulated/0/Android/data/com.acme.app/files/rooms.db"},
	}}
	c, dev := searchCoordinator(f)

	got, err := c.SearchFiles(context.Background(), dev, AppStorage("com.acme.app"), "rooms", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(f.calls) != 2 {
		t.Errorf("searched %v, want both roots", f.calls)
	}
	if len(got) != 2 {
		t.Fatalf("got %d matches, want 2 (the shared path once)", len(got))
	}
}

func TestSearchFilesOrdersShallowestFirst(t *testing.T) {
	f := &fakeSearcher{byRoot: map[string][]string{
		"/r": {
			"/r/a/b/c/deep-rooms.db",
			"/r/rooms.db",
			"/r/a/rooms.db",
		},
	}}
	c, dev := searchCoordinator(f)

	got, _ := c.SearchFiles(context.Background(), dev, Directory("/r"), "rooms", 0)

	var paths []string
	for _, m := range got {
		paths = append(paths, m.Path)
	}
	want := []string{"/r/rooms.db", "/r/a/rooms.db", "/r/a/b/c/deep-rooms.db"}
	for i := range want {
		if i >= len(paths) || paths[i] != want[i] {
			t.Fatalf("got %v, want nearest-first %v", paths, want)
		}
	}
}

func TestSearchFilesRespectsLimitAcrossRoots(t *testing.T) {
	f := &fakeSearcher{byRoot: map[string][]string{
		"/data/data/multi":                       {"/data/data/multi/rooms1.db", "/data/data/multi/rooms2.db", "/data/data/multi/rooms3.db"},
		"/storage/emulated/0/Android/data/multi": {"/storage/emulated/0/Android/data/multi/rooms4.db", "/storage/emulated/0/Android/data/multi/rooms5.db"},
	}}
	c, dev := searchCoordinator(f)

	got, _ := c.SearchFiles(context.Background(), dev, AppStorage("multi"), "rooms", 4)
	if len(got) != 4 {
		t.Errorf("got %d matches, want the limit of 4", len(got))
	}
}

func TestSearchFilesSurvivesAnUnreadableRoot(t *testing.T) {
	// One refused root must not hide what the others found; that is the
	// normal case for a release-signed app's private storage.
	f := &failingFirstRoot{ok: map[string][]string{
		"/storage/emulated/0/Android/data/multi": {"/storage/emulated/0/Android/data/multi/rooms.db"},
	}}
	c := NewCoordinator(f)
	dev := Device{ID: "X", Platform: PlatformAndroid, Kind: KindVirtual}

	got, err := c.SearchFiles(context.Background(), dev, AppStorage("multi"), "rooms", 0)
	if err != nil {
		t.Fatalf("one failing root must not fail the search: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/storage/emulated/0/Android/data/multi/rooms.db" {
		t.Errorf("got %+v, want the readable root's match", got)
	}
}

type failingFirstRoot struct{ ok map[string][]string }

func (f *failingFirstRoot) ListDevices(context.Context) ([]Device, error) { return nil, nil }
func (f *failingFirstRoot) Platform() Platform                            { return PlatformAndroid }
func (f *failingFirstRoot) Kind() DeviceKind                              { return KindVirtual }
func (f *failingFirstRoot) FileSystemFor(FileTarget, Depth) FileSystem    { return nil }

func (f *failingFirstRoot) StorageRoots(_ context.Context, _ Device, t FileTarget) ([]string, error) {
	if t.BundleID() != "" {
		return []string{"/data/data/" + t.BundleID(), "/storage/emulated/0/Android/data/" + t.BundleID()}, nil
	}
	return []string{t.Path()}, nil
}

func (f *failingFirstRoot) SearchFiles(_ context.Context, _ Device, root, _ string, _ int) ([]FileMatch, error) {
	paths, ok := f.ok[root]
	if !ok {
		return nil, context.DeadlineExceeded
	}
	var out []FileMatch
	for _, p := range paths {
		out = append(out, FileMatch{Name: gopath.Base(p), Path: p})
	}
	return out, nil
}

func TestSearchFilesReportsFailureWhenNothingIsReadable(t *testing.T) {
	c := NewCoordinator(&failingFirstRoot{})
	dev := Device{ID: "X", Platform: PlatformAndroid, Kind: KindVirtual}

	if _, err := c.SearchFiles(context.Background(), dev, Directory("/a"), "rooms", 0); err == nil {
		t.Error("expected the error when no root could be read")
	}
}

func TestSearchFilesRejectsAnEmptyQuery(t *testing.T) {
	c, dev := searchCoordinator(&fakeSearcher{})
	if _, err := c.SearchFiles(context.Background(), dev, Directory("/a"), "  ", 0); err == nil {
		t.Error("an empty query would match the whole device")
	}
}

func TestMatchesQuery(t *testing.T) {
	const path = "/data/data/com.acme.app/databases/rooms.db"

	tests := []struct {
		query string
		want  bool
		why   string
	}{
		{"rooms", true, "plain substring of the name"},
		{"ROOMS", true, "case-insensitive"},
		{"db", true, "extension"},
		{"nope", false, "no match"},
		{"databases/rooms", true, "a query with a separator matches the path"},
		{"databases", false, "a directory name alone is not part of the file name"},
		{"com.acme.app/databases/", true, "longer path fragment"},
	}

	for _, tc := range tests {
		if got := matchesQuery("rooms.db", path, tc.query); got != tc.want {
			t.Errorf("%s: matchesQuery(%q) = %v, want %v", tc.why, tc.query, got, tc.want)
		}
	}
}

func TestEscapeBRE(t *testing.T) {
	// The query is a literal, so regex metacharacters have to lose their
	// meaning — grep -F is not usable here (toybox treats it as match-all).
	tests := []struct{ in, want string }{
		{"rooms", "rooms"},
		{"rooms.db", `rooms\.db`},
		{"a*b", `a\*b`},
		{"x[0]", `x\[0\]`},
		{"^start", `\^start`},
		{"end$", `end\$`},
		{`back\slash`, `back\\slash`},
		{"g.test3@cmms-test.com#App", `g\.test3@cmms-test\.com#App`},
	}

	for _, tc := range tests {
		if got := escapeBRE(tc.in); got != tc.want {
			t.Errorf("escapeBRE(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAndroidRunAsForPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		// The package is taken from the path, so a search needs no extra
		// argument to reach private storage.
		{"/data/data/com.acme.app/databases/rooms.db", "run-as 'com.acme.app' "},
		{"/data/user/0/com.acme.app/files", "run-as 'com.acme.app' "},
		{"/data/user_de/0/com.acme.app", "run-as 'com.acme.app' "},

		{"/storage/emulated/0/Android/data/com.acme.app", ""},
		{"/sdcard/Download", ""},
		{"/data/data/", ""},
		{"", ""},
	}

	for _, tc := range tests {
		if got := androidRunAsForPath(tc.path); got != tc.want {
			t.Errorf("androidRunAsForPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestDevicePathsUseSlashSemantics(t *testing.T) {
	// Device paths are slash-separated whatever this host is, which is what
	// the stdlib "path" package means — "path/filepath" would be wrong on
	// Windows.
	for in, want := range map[string]string{
		"/a/b/c.db":   "c.db",
		"/a/b/":       "b",
		"rooms.db":    "rooms.db",
		"/data/data/": "data",
	} {
		if got := gopath.Base(in); got != want {
			t.Errorf("path.Base(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIOSSimulatorSearchWalksTheHostTree(t *testing.T) {
	// A simulator's storage is a directory on this Mac, so its search is a
	// local walk and can be exercised for real.
	dir := t.TempDir()
	mustWrite(t, dir+"/rooms.db")
	mustWrite(t, dir+"/notes.txt")
	mustMkdir(t, dir+"/Library")
	mustWrite(t, dir+"/Library/other-rooms.sqlite")

	m := &iosManager{}
	got, err := m.SearchFiles(context.Background(), Device{Platform: PlatformIOS}, dir, "rooms", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	var names []string
	for _, g := range got {
		names = append(names, g.Name)
	}
	if len(got) != 2 {
		t.Fatalf("got %v, want both rooms files", names)
	}
	for _, n := range names {
		if !strings.Contains(strings.ToLower(n), "rooms") {
			t.Errorf("unexpected match %q", n)
		}
	}
}

func TestIOSSimulatorSearchHonoursTheLimit(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"rooms1.db", "rooms2.db", "rooms3.db"} {
		mustWrite(t, dir+"/"+n)
	}

	m := &iosManager{}
	got, err := m.SearchFiles(context.Background(), Device{Platform: PlatformIOS}, dir, "rooms", 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d matches, want the limit of 2", len(got))
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
