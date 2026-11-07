package dbviewer

import (
	"testing"

	"simmer/internal/config"

	tea "charm.land/bubbletea/v2"
)

func TestSettings_VimToggle(t *testing.T) {
	cfg := config.Default()
	s := newSettings(cfg, "app.db")
	if !s.vimMode {
		t.Fatal("vim mode should start from the config (on by default)")
	}

	// Script → Query → Vim
	for range 2 {
		s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	if s.focus != settingsFocusVim {
		t.Fatalf("focus = %v, want the vim toggle", s.focus)
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if s.vimMode {
		t.Error("space did not turn vim mode off")
	}
	s, _ = s.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !s.vimMode {
		t.Error("enter did not turn vim mode back on")
	}

	// Space elsewhere is text, not a toggle.
	s, _ = s.applyFocus(settingsFocusScript)
	s, _ = s.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if !s.vimMode {
		t.Error("space in the script path toggled vim mode")
	}
}
