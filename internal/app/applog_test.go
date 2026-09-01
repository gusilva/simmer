package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"simmer/internal/device"
)

func TestLogFileName(t *testing.T) {
	cases := []struct {
		name string
		app  device.App
		want string
	}{
		{"plain", device.App{DisplayName: "Calendar"}, "Calendar.log"},
		{"spaces collapse", device.App{DisplayName: "Mobile Safari"}, "Mobile_Safari.log"},
		{"reserved chars", device.App{Name: "a/b:c*d"}, "a_b_c_d.log"},
		{"trims edge underscores", device.App{Name: " / weird / "}, "weird.log"},
		{"falls back to bundle id", device.App{BundleID: "com.apple.mobilecal"}, "com.apple.mobilecal.log"},
		{"empty everything", device.App{}, "app.log"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := logFileName(c.app); got != c.want {
				t.Errorf("logFileName(%+v) = %q, want %q", c.app, got, c.want)
			}
		})
	}
}

func TestStartAppLogging_WritesHeaderAndTruncates(t *testing.T) {
	dir := t.TempDir()
	m := &model{launchDir: dir}
	dev := device.Device{Name: "iPhone 15"}
	app := device.App{DisplayName: "Calendar", BundleID: "com.apple.mobilecal"}

	path := filepath.Join(dir, "Calendar.log")
	if err := os.WriteFile(path, []byte("stale contents from a previous run\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := m.startAppLogging(dev, app)
	if err != nil {
		t.Fatalf("startAppLogging: %v", err)
	}
	m.logFile = f
	m.closeLogFile()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if strings.Contains(got, "stale contents") {
		t.Error("file was not truncated")
	}
	if !strings.HasPrefix(got, "# Calendar (com.apple.mobilecal) on iPhone 15 — ") {
		t.Errorf("missing/incorrect header line: %q", got)
	}
}

func TestCloseLogFile_Idempotent(t *testing.T) {
	m := &model{}
	m.closeLogFile() // nil file — must not panic

	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	m.logFile = f
	m.closeLogFile()
	m.closeLogFile() // second call — must not panic
	if m.logFile != nil {
		t.Error("logFile must be nil after close")
	}
}
