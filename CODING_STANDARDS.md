# Coding standards

Judgement calls for review. Anything a tool can decide lives in `make check` instead.

## Tests

**A regression test is seen red.** A test added for a bug comes with evidence it fails without the
fix — the failure output in the PR, or a note of how the fix was disabled and what failed. A test
that only ever ran green may pass for the wrong reason: an assertion that cannot fail, a message
the component ignores, a command that never ran.

**Bubble Tea commands run through `uitest.Settle`.** A test that calls a returned `tea.Cmd` inline
drops whatever that command would have delivered later — goeditor's signal listener blocks until
the editor signals, so inline it loses the signal. `internal/ui/uitest` runs commands the way the
runtime does; a hand-rolled loop over `cmd()` in a UI test is a finding.
