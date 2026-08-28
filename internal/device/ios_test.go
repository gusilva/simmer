package device

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseSimctlOutput(t *testing.T) {
	jsonInput := `{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-0": [
				{
					"state": "Booted",
					"isAvailable": true,
					"name": "iPhone 15",
					"udid": "123-456"
				},
				{
					"state": "Shutdown",
					"isAvailable": true,
					"name": "iPhone 14",
					"udid": "789-012"
				}
			]
		}
	}`

	devices, err := parseSimctlOutput([]byte(jsonInput))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}

	foundRunning := false
	for _, d := range devices {
		if d.Name == "iPhone 15" && d.Status == StatusRunning {
			foundRunning = true
		}
	}

	if !foundRunning {
		t.Error("expected to find running iPhone 15")
	}
}

func TestResolveIOSProjectPath(t *testing.T) {
	if _, err := exec.LookPath("plutil"); err != nil {
		t.Skip("plutil not available")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	if err := os.MkdirAll(filepath.Join(derivedData, "MyApp-abc123"), 0o755); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join(home, "src", "MyApp", "MyApp.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Includes a <date> field (LastAccessedDate), matching real DerivedData
	// info.plist files — plutil -convert json rejects Date values, which is
	// why ResolveIOSProjectPath must use plutil -p instead.
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>LastAccessedDate</key>
	<date>2026-07-24T20:52:58Z</date>
	<key>WorkspacePath</key>
	<string>` + projectPath + `</string>
</dict>
</plist>`
	if err := os.WriteFile(filepath.Join(derivedData, "MyApp-abc123", "info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveIOSProjectPath("MyApp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != projectPath {
		t.Errorf("got %q, want %q", got, projectPath)
	}
}

func TestParsePlutilWorkspacePath(t *testing.T) {
	output := `{
  "LastAccessedDate" => 2026-07-24 20:52:58 +0000
  "WorkspacePath" => "/Users/gustavo/Documents/teaching/convert-heic/convert-heic/convert-heic.xcodeproj"
}
`
	got, ok := parsePlutilWorkspacePath(output)
	if !ok {
		t.Fatal("expected WorkspacePath to be found")
	}
	want := "/Users/gustavo/Documents/teaching/convert-heic/convert-heic/convert-heic.xcodeproj"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParsePlutilWorkspacePath_NotFound(t *testing.T) {
	output := `{
  "LastAccessedDate" => 2026-07-24 20:52:58 +0000
}
`
	if _, ok := parsePlutilWorkspacePath(output); ok {
		t.Error("expected not found")
	}
}

func TestResolveIOSProjectPath_NoMatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	if err := os.MkdirAll(derivedData, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveIOSProjectPath("NoSuchApp"); err == nil {
		t.Error("expected error when no DerivedData folder matches")
	}
}
