package logging

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestLogger(t *testing.T, level Level) *Logger {
	t.Helper()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	l, err := New(level)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Close)
	return l
}

func readLog(t *testing.T, l *Logger) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(".", "simmer.log"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLogError_WritesErrorLine(t *testing.T) {
	l := newTestLogger(t, LevelError)
	l.LogError("resolve ios project for MyApp", errors.New("no DerivedData folder found"))

	got := readLog(t, l)
	if !strings.Contains(got, "ERROR") {
		t.Errorf("expected ERROR label in log, got %q", got)
	}
	if !strings.Contains(got, "resolve ios project for MyApp") {
		t.Errorf("expected op context in log, got %q", got)
	}
	if !strings.Contains(got, "no DerivedData folder found") {
		t.Errorf("expected error message in log, got %q", got)
	}
}

func TestLogError_NilErrorIsNoOp(t *testing.T) {
	l := newTestLogger(t, LevelError)
	l.LogError("resolve ios project for MyApp", nil)

	got := readLog(t, l)
	if got != "" {
		t.Errorf("expected no log output for nil error, got %q", got)
	}
}

func TestLogError_NilLoggerIsNoOp(t *testing.T) {
	var l *Logger
	l.LogError("resolve ios project for MyApp", errors.New("boom"))
}
