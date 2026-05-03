package ui

import (
	"os/exec"
	"testing"
)

func TestCopyToClipboardCmd(t *testing.T) {
	cmd := CopyToClipboardCmd("test text")
	if cmd == nil {
		t.Fatal("expected command, got nil")
	}

	// We can't easily test the execution without pbcopy,
	// but we can verify it returns a message.
	// In CI, pbcopy might be missing, so we just check the return type.

	msg := cmd()
	m, ok := msg.(ClipboardCopiedMsg)
	if !ok {
		t.Errorf("expected ClipboardCopiedMsg, got %T", msg)
	}
	if m.Text != "test text" {
		t.Errorf("expected text 'test text', got %q", m.Text)
	}

	// If pbcopy is missing, Err should be non-nil
	if _, err := exec.LookPath("pbcopy"); err != nil {
		if m.Err == nil {
			t.Error("expected error when pbcopy is missing")
		}
	}
}
