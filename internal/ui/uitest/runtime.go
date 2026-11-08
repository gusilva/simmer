// Package uitest runs Bubble Tea commands in tests the way the runtime does.
//
// Calling a returned tea.Cmd inline is not enough for components that keep a
// command blocked until something happens — goeditor's signal listener waits
// on a channel for the editor's next signal. Run inline with a timeout, that
// command is abandoned while still waiting, swallows the signal it was waiting
// for, and a test passes without the message flow it meant to exercise.
// Settle keeps every command running and collects its message whenever it
// comes, as the runtime would.
package uitest

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// Quiet is how long Settle waits without a new message before it decides the
// model has settled. Debounce timers in a test should be shorter than this.
const Quiet = 50 * time.Millisecond

// Updater is a model whose Update returns its own type, as Bubble Tea
// sub-models do.
type Updater[M any] interface {
	Update(tea.Msg) (M, tea.Cmd)
}

// Settle feeds the messages cmds produce back into m — and the messages the
// commands that returns produce, and so on — until none arrive for Quiet.
// Batches are expanded. A message for which intercept returns true is not fed
// back: it is meant for m's parent (a close request, say) and is returned.
func Settle[M Updater[M]](m M, cmds []tea.Cmd, intercept func(tea.Msg) bool) (M, []tea.Msg) {
	out := make(chan tea.Msg, 256)
	start := func(cmd tea.Cmd) {
		if cmd != nil {
			go func() { out <- cmd() }()
		}
	}
	for _, c := range cmds {
		start(c)
	}

	var intercepted []tea.Msg
	for {
		select {
		case msg := <-out:
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					start(c)
				}
				continue
			}
			if msg == nil {
				continue
			}
			if intercept != nil && intercept(msg) {
				intercepted = append(intercepted, msg)
				continue
			}
			var cmd tea.Cmd
			m, cmd = m.Update(msg)
			start(cmd)
		case <-time.After(Quiet):
			return m, intercepted
		}
	}
}
