package ui

import tea "charm.land/bubbletea/v2"

// dbPane is the common contract every DB viewer pane must satisfy.
// The modal dispatches to panes by index, not by type-switch, so adding
// a new pane only requires registering it in NewDBViewerModal.
type dbPane interface {
	Update(tea.Msg) (dbPane, tea.Cmd)
	Rows(width, height int, focused bool) []string
	Focus() (dbPane, tea.Cmd)
	Blur() dbPane
}

// ── DBExplorer adapter ────────────────────────────────────────────────────────

func (e DBExplorer) asPane() dbPane { return explorerPane{e} }

type explorerPane struct{ inner DBExplorer }

func (p explorerPane) Update(msg tea.Msg) (dbPane, tea.Cmd) {
	next, cmd := p.inner.Update(msg)
	return explorerPane{next}, cmd
}

func (p explorerPane) Rows(width, height int, _ bool) []string {
	return p.inner.Rows(width, height)
}

func (p explorerPane) Focus() (dbPane, tea.Cmd) { return p, nil }
func (p explorerPane) Blur() dbPane              { return p }

// ── DBQueryPane adapter ───────────────────────────────────────────────────────

func (q DBQueryPane) asPane() dbPane { return queryPane{q} }

type queryPane struct{ inner DBQueryPane }

func (p queryPane) Update(msg tea.Msg) (dbPane, tea.Cmd) {
	next, cmd := p.inner.Update(msg)
	return queryPane{next}, cmd
}

func (p queryPane) Rows(width, height int, focused bool) []string {
	return p.inner.Rows(width, height, focused)
}

func (p queryPane) Focus() (dbPane, tea.Cmd) {
	next, cmd := p.inner.FocusEditor()
	return queryPane{next}, cmd
}

func (p queryPane) Blur() dbPane {
	return queryPane{p.inner.BlurEditor()}
}

// ── DBResultsPane adapter ─────────────────────────────────────────────────────

func (r DBResultsPane) asPane() dbPane { return resultsPane{r} }

type resultsPane struct{ inner DBResultsPane }

func (p resultsPane) Update(msg tea.Msg) (dbPane, tea.Cmd) {
	next, cmd := p.inner.Update(msg)
	return resultsPane{next}, cmd
}

func (p resultsPane) Rows(width, height int, focused bool) []string {
	return p.inner.Rows(width, height, focused)
}

func (p resultsPane) Focus() (dbPane, tea.Cmd) { return p, nil }
func (p resultsPane) Blur() dbPane              { return p }
