package device

import (
	"strings"
	"testing"
)

// ---------- parseAVDManagerDevices ----------

func TestParseAVDManagerDevices_Basic(t *testing.T) {
	input := []byte(`Available Android Virtual Devices:
    id: 1 or "pixel_6"
    Name: Pixel 6
    OEM : Google
---------
    id: 2 or "nexus_5x"
    Name: Nexus 5X
    OEM : Google
`)
	devices := parseAVDManagerDevices(input)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices[0].Identifier != "pixel_6" {
		t.Errorf("id[0]: got %q, want pixel_6", devices[0].Identifier)
	}
	if devices[0].Name != "Pixel 6" {
		t.Errorf("name[0]: got %q, want Pixel 6", devices[0].Name)
	}
	if devices[1].Identifier != "nexus_5x" {
		t.Errorf("id[1]: got %q, want nexus_5x", devices[1].Identifier)
	}
}

func TestParseAVDManagerDevices_Empty(t *testing.T) {
	devices := parseAVDManagerDevices([]byte(""))
	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestParseAVDManagerDevices_NoSeparator(t *testing.T) {
	// Last entry has no trailing separator line — must still be captured.
	input := []byte(`    id: 1 or "only_one"
    Name: Only One
`)
	devices := parseAVDManagerDevices(input)
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].Identifier != "only_one" {
		t.Errorf("id: got %q", devices[0].Identifier)
	}
}

// ---------- isLsDirHeader ----------

func TestIsLsDirHeader(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{".", false},            // no trailing colon
		{".:", true},           // root
		{"./foo:", true},       // subdirectory
		{"./foo/bar:", true},   // deeper
		{"/absolute/path:", false}, // not relative
		{"total 24", false},
		{"-rw-rw-r-- 1 root root 0 2024-01-01 00:00 file.txt", false},
		{"  ./:  ", true}, // trimmed
	}
	for _, tc := range tests {
		got := isLsDirHeader(tc.line)
		if got != tc.want {
			t.Errorf("isLsDirHeader(%q) = %v, want %v", tc.line, got, tc.want)
		}
	}
}

// ---------- parseLsLine ----------

func TestParseLsLine_RegularFile(t *testing.T) {
	line := "-rw-rw-r-- 1 root root 1234 2024-01-15 10:30 myfile.txt"
	e, ok := parseLsLine(line, "")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if e.relPath != "myfile.txt" {
		t.Errorf("relPath: got %q", e.relPath)
	}
	if e.isDir {
		t.Error("expected isDir=false")
	}
	if e.size != 1234 {
		t.Errorf("size: got %d, want 1234", e.size)
	}
}

func TestParseLsLine_Directory(t *testing.T) {
	line := "drwxrwxr-x 2 root root 4096 2024-01-15 10:30 subdir"
	e, ok := parseLsLine(line, "parent")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !e.isDir {
		t.Error("expected isDir=true")
	}
	if e.relPath != "parent/subdir" {
		t.Errorf("relPath: got %q, want parent/subdir", e.relPath)
	}
}

func TestParseLsLine_SkipDotEntries(t *testing.T) {
	for _, name := range []string{".", ".."} {
		line := "drwxr-xr-x 2 root root 4096 2024-01-15 10:30 " + name
		_, ok := parseLsLine(line, "")
		if ok {
			t.Errorf("expected ok=false for %q", name)
		}
	}
}

func TestParseLsLine_SymlinkStripped(t *testing.T) {
	line := "lrwxrwxrwx 1 root root 10 2024-01-15 10:30 link -> /target/path"
	e, ok := parseLsLine(line, "")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if e.relPath != "link" {
		t.Errorf("relPath: got %q, want link", e.relPath)
	}
}

func TestParseLsLine_TooFewFields(t *testing.T) {
	_, ok := parseLsLine("drwxr-xr-x 2 root", "")
	if ok {
		t.Error("expected ok=false for too-few-fields line")
	}
}

func TestParseLsLine_InvalidPermChar(t *testing.T) {
	line := "?rwxr-xr-x 2 root root 0 2024-01-15 10:30 file"
	_, ok := parseLsLine(line, "")
	if ok {
		t.Error("expected ok=false for invalid perm char")
	}
}

// ---------- parseLsLaR ----------

func TestParseLsLaR_BasicTree(t *testing.T) {
	data := `.:
drwxr-xr-x 3 root root 4096 2024-01-15 10:30 shared_prefs
-rw-rw-r-- 1 root root  512 2024-01-15 10:31 databases

./shared_prefs:
-rw-rw-r-- 1 root root  200 2024-01-15 10:32 prefs.xml
`
	root := parseLsLaR(data, "com.example.app", 10)
	if root.Name == "" {
		t.Error("expected non-empty root name")
	}
	if len(root.Children) == 0 {
		t.Error("expected children in root")
	}
}

func TestParseLsLaR_MaxDepthRespected(t *testing.T) {
	// With maxDepth=1, entries under ./subdir should be skipped.
	data := `.:
drwxr-xr-x 2 root root 4096 2024-01-15 10:30 subdir

./subdir:
-rw-r--r-- 1 root root 100 2024-01-15 10:30 hidden.txt
`
	root := parseLsLaR(data, "com.example", 1)
	// The subdir entry itself should exist but its child (hidden.txt) should not.
	var foundSubdir bool
	for _, c := range root.Children {
		if c.Name == "subdir" {
			foundSubdir = true
			if len(c.Children) > 0 {
				t.Error("expected no children inside subdir when maxDepth=1")
			}
		}
	}
	if !foundSubdir {
		t.Error("expected subdir to be present")
	}
}

// ---------- apiFromSysdir ----------

func TestAPIFromSysdir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"system-images/android-33/google_apis/x86_64/", "33"},
		{"system-images/android-34/google_apis_playstore/arm64-v8a/", "34"},
		{"", ""},
		{"no-android-prefix/here/", ""},
		{"android-/empty/", ""}, // empty after prefix
	}
	for _, tc := range tests {
		got := apiFromSysdir(tc.input)
		if got != tc.want {
			t.Errorf("apiFromSysdir(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------- formatPartitionSize ----------

func TestFormatPartitionSize(t *testing.T) {
	tests := []struct {
		input string
		want  string // partial match via Contains
	}{
		{"512K", "KB"},
		{"2048M", "MB"},
		{"8G", "GB"},
		{"512k", "KB"},
		{"1024m", "MB"},
		{"", ""},
		{"0", "0"}, // zero bytes returns raw
	}
	for _, tc := range tests {
		got := formatPartitionSize(tc.input)
		if tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("formatPartitionSize(%q) = %q, want to contain %q", tc.input, got, tc.want)
		}
		if tc.want == "" && got != "" {
			t.Errorf("formatPartitionSize(%q) = %q, want empty", tc.input, got)
		}
	}
}
