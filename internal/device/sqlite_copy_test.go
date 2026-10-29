package device

import (
	"errors"
	"testing"
)

func TestIsMissingSQLite3(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		// Real messages observed on devices without the binary.
		{"toybox not found", "/system/bin/sh: sqlite3: not found", true},
		{"miui inaccessible", "/system/bin/sh: sqlite3: inaccessible or not found", true},
		{"run-as exec failed", "run-as: exec failed for sqlite3: Permission denied", true},
		{"no such file", "sh: sqlite3: No such file or directory", true},

		// Query and file problems must not be mistaken for a missing binary,
		// or a genuine SQL error would be retried against a local copy and
		// reported from there.
		{"sql error", "Error: near \"slect\": syntax error", false},
		{"missing table", "Error: no such table: rooms", false},
		{"locked", "Error: database is locked", false},
		{"unrelated permission denied", "run-as: package not debuggable com.acme.app", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		if got := isMissingSQLite3Text(tc.msg); got != tc.want {
			t.Errorf("%s: isMissingSQLite3Text(%q) = %v, want %v", tc.name, tc.msg, got, tc.want)
		}
		if got := isMissingSQLite3(errors.New(tc.msg)); tc.msg != "" && got != tc.want {
			t.Errorf("%s: isMissingSQLite3(err) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsMissingSQLite3IgnoresNil(t *testing.T) {
	if isMissingSQLite3(nil) {
		t.Error("a nil error is not a missing binary")
	}
}

func TestPhysicalAndroidSandboxIsReadLikeAnEmulator(t *testing.T) {
	m := &physicalAndroidManager{}

	// A sandbox request must reach the app's own storage, not the shared
	// storage that an earlier fallback answered with — that fallback turned
	// "this app's databases" into "whatever is on the sdcard", which
	// surfaced as an empty list.
	sandbox, ok := m.FileSystemFor(AppStorage("com.acme.app"), FullDepth).(*AndroidFileSystem)
	if !ok {
		t.Fatalf("sandbox scope returned %T, want the app-storage FileSystem", m.FileSystemFor(AppStorage("com.acme.app"), FullDepth))
	}
	if sandbox.PackageID != "com.acme.app" {
		t.Errorf("sandbox is scoped to %q", sandbox.PackageID)
	}

	// The device's own filesystem, and a target naming nothing, list shared
	// storage — the latter never reaches an adapter in practice because
	// FileTree rejects it, but the provider must still be total.
	for _, fs := range []FileSystem{
		m.FileSystemFor(DeviceRoot(), FullDepth),
		m.FileSystemFor(FileTarget{}, FullDepth),
	} {
		if _, ok := fs.(*AndroidPhysicalFileSystem); !ok {
			t.Errorf("got %T, want shared storage", fs)
		}
	}
}
