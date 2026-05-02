package ui

import (
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ClipboardCopiedMsg is dispatched after a CopyToClipboardCmd completes.
// Err is non-nil if the underlying clipboard write failed.
type ClipboardCopiedMsg struct {
	Text string
	Err  error
}

// CopyToClipboardCmd returns a tea.Cmd that writes text to the system
// clipboard via macOS `pbcopy`. The resulting message is a ClipboardCopiedMsg.
func CopyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return ClipboardCopiedMsg{Text: text, Err: err}
		}
		return ClipboardCopiedMsg{Text: text}
	}
}
