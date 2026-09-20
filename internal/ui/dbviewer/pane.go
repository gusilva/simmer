package dbviewer

import tea "charm.land/bubbletea/v2"

// pane is the common contract every DB viewer pane must satisfy.
// The modal dispatches to panes by index, not by type-switch.
type pane interface {
	Update(tea.Msg) (pane, tea.Cmd)
	Rows(width, height int, focused bool) []string
	Focus() (pane, tea.Cmd)
	Blur() pane
}

// ── Explorer adapter ──────────────────────────────────────────────────────────

func (e Explorer) asPane() pane { return explorerPane{e} }

type explorerPane struct{ inner Explorer }

func (p explorerPane) Update(msg tea.Msg) (pane, tea.Cmd) {
	next, cmd := p.inner.Update(msg)
	return explorerPane{next}, cmd
}

func (p explorerPane) Rows(width, height int, focused bool) []string {
	return p.inner.Rows(width, height, focused)
}

func (p explorerPane) Focus() (pane, tea.Cmd) { return p, nil }
func (p explorerPane) Blur() pane             { return p }

// ── QueryPane adapter ─────────────────────────────────────────────────────────

func (q QueryPane) asPane() pane { return queryPaneAdapter{q} }

type queryPaneAdapter struct{ inner QueryPane }

func (p queryPaneAdapter) Update(msg tea.Msg) (pane, tea.Cmd) {
	next, cmd := p.inner.Update(msg)
	return queryPaneAdapter{next}, cmd
}

func (p queryPaneAdapter) Rows(width, height int, focused bool) []string {
	return p.inner.Rows(width, height, focused)
}

func (p queryPaneAdapter) Focus() (pane, tea.Cmd) {
	next, cmd := p.inner.FocusEditor()
	return queryPaneAdapter{next}, cmd
}

func (p queryPaneAdapter) Blur() pane {
	return queryPaneAdapter{p.inner.BlurEditor()}
}

// ── ResultsPane adapter ───────────────────────────────────────────────────────

func (r ResultsPane) asPane() pane { return resultsPaneAdapter{r} }

type resultsPaneAdapter struct{ inner ResultsPane }

func (p resultsPaneAdapter) Update(msg tea.Msg) (pane, tea.Cmd) {
	next, cmd := p.inner.Update(msg)
	return resultsPaneAdapter{next}, cmd
}

func (p resultsPaneAdapter) Rows(width, height int, focused bool) []string {
	return p.inner.Rows(width, height, focused)
}

func (p resultsPaneAdapter) Focus() (pane, tea.Cmd) { return p, nil }
func (p resultsPaneAdapter) Blur() pane             { return p }
