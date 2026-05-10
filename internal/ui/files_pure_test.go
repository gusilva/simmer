package ui

import (
	"strings"
	"testing"
)

// ---------- formatSize ----------

func TestFormatSize_Bytes(t *testing.T) {
	got := formatSize(500)
	if !strings.Contains(got, "B") {
		t.Errorf("expected B suffix, got %q", got)
	}
}

func TestFormatSize_Kilobytes(t *testing.T) {
	got := formatSize(5_000)
	if !strings.Contains(got, "KB") {
		t.Errorf("expected KB, got %q", got)
	}
}

func TestFormatSize_Megabytes(t *testing.T) {
	got := formatSize(5_000_000)
	if !strings.Contains(got, "MB") {
		t.Errorf("expected MB, got %q", got)
	}
}

func TestFormatSize_Gigabytes(t *testing.T) {
	got := formatSize(5_000_000_000)
	if !strings.Contains(got, "GB") {
		t.Errorf("expected GB, got %q", got)
	}
}

// ---------- fileExtSuffix ----------

func TestFileExtSuffix_WithExt(t *testing.T) {
	got := fileExtSuffix("archive.zip")
	if got != "(.zip)" {
		t.Errorf("got %q, want (.zip)", got)
	}
}

func TestFileExtSuffix_NoExt(t *testing.T) {
	got := fileExtSuffix("Makefile")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFileExtSuffix_DotFile(t *testing.T) {
	got := fileExtSuffix(".gitignore")
	if !strings.HasPrefix(got, "(") {
		t.Errorf("expected parens, got %q", got)
	}
}

// ---------- padRight ----------

func TestPadRight_ShortString(t *testing.T) {
	got := padRight("hi", 6)
	if len(got) != 6 {
		t.Errorf("expected len 6, got %d: %q", len(got), got)
	}
}

func TestPadRight_ExactLength(t *testing.T) {
	got := padRight("hello", 5)
	if got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestPadRight_LongerThanTarget(t *testing.T) {
	got := padRight("toolong", 3)
	if got != "toolong" {
		t.Errorf("expected unchanged string, got %q", got)
	}
}
