package ui

import (
	gopath "path"
	"strconv"
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Finding a file in the Files panel comes in two parts, because the tree
// loads one directory at a time and so knows only what has been expanded.
//
//   - A filter ("/") narrows the rows already on screen. It is instant and
//     costs nothing, but cannot see a file in a directory nobody opened.
//   - A search (ctrl+f) asks the device, and finds everything under the
//     tree's roots. It costs a round-trip and returns a flat list of paths.
//
// The filter bar names the second one as soon as the first is in use, so a
// query that comes up empty is not mistaken for a file that is not there.

// fileSearchState is the device-search half: a flat result list that
// replaces the tree until dismissed.
type fileSearchState struct {
	active  bool
	query   string
	running bool
	results []device.FileMatch
	idx     int
	err     string
}

// RequestFileSearchMsg asks the parent to search the device over whatever
// the current tree covers.
type RequestFileSearchMsg struct {
	Target device.FileTarget
	Query  string
}

// RequestFileTreeAtMsg asks the parent to re-root the tree at Path. Used
// when a search result is a directory: the result list is a way of getting
// somewhere, so picking one goes there.
type RequestFileTreeAtMsg struct {
	Path string
}

// ── filter over the loaded tree ─────────────────────────────────────────

// visibleTreeRows returns the rows to draw: the expanded tree, narrowed by
// the active filter.
//
// A row is kept when it matches, when one of its ancestors matches, or when
// one of its descendants does. The three cases are what make a filtered tree
// behave like a tree:
//
//   - the match itself, obviously;
//   - its ancestors, so a match is shown in its place rather than as an
//     orphan;
//   - its descendants, so a matched directory can be opened. Without this a
//     folder whose own name matched would expand to nothing — the caret
//     said open, the preview said two entries, and the pane showed neither.
func (m MainPane) visibleTreeRows() []treeRow {
	rows := m.flattenTree()
	if m.filesFilter == "" {
		return rows
	}

	query := strings.ToLower(m.filesFilter)
	parent := treeParents(rows)

	keep := make([]bool, len(rows))

	// Forward: a match, or anything under one.
	underMatch := make([]bool, len(rows))
	for i, r := range rows {
		matched := matchesFilter(query, r.node.Name)
		underMatch[i] = matched || (parent[i] >= 0 && underMatch[parent[i]])
		keep[i] = underMatch[i]

		// The match's ancestors, walked up from it.
		if matched {
			for p := parent[i]; p >= 0 && !keep[p]; p = parent[p] {
				keep[p] = true
			}
		}
	}

	out := make([]treeRow, 0, len(rows))
	for i, r := range rows {
		if keep[i] {
			out = append(out, r)
		}
	}
	return out
}

// matchesFilter is the filter's matching rule: a case-insensitive
// substring, deliberately the same rule the device search applies.
//
// The filter is an on-ramp to that search — ctrl+f hands it the text already
// typed — so a fuzzy match here would accept queries the device then finds
// nothing for, which is the confusion the escalation exists to prevent.
func matchesFilter(query, name string) bool {
	return strings.Contains(strings.ToLower(name), query)
}

// treeParents returns each row's parent index, or -1 for a top-level row.
// Rows arrive in depth-first order, so the parent of a row at depth d is the
// most recent row at depth d-1.
func treeParents(rows []treeRow) []int {
	parent := make([]int, len(rows))
	var path []int // path[d] = index of the row at depth d above the cursor

	for i, r := range rows {
		if r.depth < len(path) {
			path = path[:r.depth]
		}
		if r.depth == 0 || len(path) == 0 {
			parent[i] = -1
		} else {
			parent[i] = path[len(path)-1]
		}
		path = append(path, i)
	}

	return parent
}

// FilterActive reports whether the filter input is capturing keys.
func (m MainPane) FilterActive() bool { return m.filesFiltering }

// ── device search ───────────────────────────────────────────────────────

// StartFileSearch enters search mode and marks it in flight.
func (m *MainPane) StartFileSearch(query string) {
	m.fileSearch = fileSearchState{active: true, query: query, running: true}
}

// SetFileSearchResults records a finished search.
func (m *MainPane) SetFileSearchResults(query string, results []device.FileMatch) {
	m.fileSearch.active = true
	m.fileSearch.query = query
	m.fileSearch.running = false
	m.fileSearch.results = results
	m.fileSearch.err = ""
	m.fileSearch.idx = 0
}

// SetFileSearchError records a failed search, keeping the mode open so the
// message is visible.
func (m *MainPane) SetFileSearchError(query, msg string) {
	m.fileSearch.active = true
	m.fileSearch.query = query
	m.fileSearch.running = false
	m.fileSearch.results = nil
	m.fileSearch.err = msg
}

// ExitFileSearch returns to the tree, clearing the filter with it. The
// query was handed to the device search, so leaving it applied would narrow
// the tree by a term the filter bar is no longer showing — which reads as a
// tree that refuses to open.
func (m *MainPane) ExitFileSearch() {
	m.fileSearch = fileSearchState{}
	m.filesFilter = ""
	m.filesFiltering = false
	m.treeIdx = 0
}

// SearchActive reports whether search results are showing instead of the tree.
func (m MainPane) SearchActive() bool { return m.fileSearch.active }

// moveSearchCursor moves the selection within the result list.
func (m *MainPane) moveSearchCursor(d int) {
	if len(m.fileSearch.results) == 0 {
		return
	}
	m.fileSearch.idx = clampIdx(m.fileSearch.idx+d, len(m.fileSearch.results))
}

// SelectedMatch returns the highlighted search result, if any.
func (m MainPane) SelectedMatch() *device.FileMatch {
	if !m.fileSearch.active || m.fileSearch.idx >= len(m.fileSearch.results) {
		return nil
	}
	return &m.fileSearch.results[m.fileSearch.idx]
}

// ActivateSearchResult acts on the highlighted result: a database opens in
// the viewer, a directory becomes the tree's new root, and any other file
// re-roots at the directory holding it — so the result list always ends in
// the tree, where the file can be previewed.
func (m *MainPane) ActivateSearchResult() tea.Cmd {
	match := m.SelectedMatch()
	if match == nil || m.active == nil {
		return nil
	}

	if !match.IsDir && device.IsDatabaseFile(match.Name) {
		dev := *m.active
		packageID := ""
		if m.selectedApp != nil {
			packageID = m.selectedApp.BundleID
		}
		path, name := match.Path, match.Name

		return func() tea.Msg {
			return ShowSQLiteViewerMsg{Device: dev, PackageID: packageID, DBPath: path, DBName: name}
		}
	}

	target := match.Path
	if !match.IsDir {
		target = gopath.Dir(match.Path)
	}
	if target == "" {
		return nil
	}

	return func() tea.Msg { return RequestFileTreeAtMsg{Path: target} }
}

// startDeviceSearch escalates the typed filter into a device search. The
// filter text carries over, so ctrl+f continues the query already typed
// rather than asking for it again.
func (m *MainPane) startDeviceSearch() tea.Cmd {
	query := strings.TrimSpace(m.filesFilter)
	if query == "" {
		query = m.fileSearch.query
	}
	if query == "" || m.tree == nil {
		return nil
	}

	m.StartFileSearch(query)
	target := m.filesTarget

	return func() tea.Msg { return RequestFileSearchMsg{Target: target, Query: query} }
}

// ── rendering ───────────────────────────────────────────────────────────

// renderSearchPane draws the flat result list that replaces the tree while a
// device search is open.
func (m MainPane) renderSearchPane(w, h int) []string {
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	errStyle := lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg)

	lines := []string{padBg(w)}

	line := func(s string) {
		pad := w - lipgloss.Width(s)
		if pad > 0 {
			s += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, s)
	}

	switch {
	case m.fileSearch.running:
		line(faint.Render("  searching the device for " + m.fileSearch.query + "…"))
	case m.fileSearch.err != "":
		line(errStyle.Render("  " + truncateName(m.fileSearch.err, max(w-4, 8))))
	case len(m.fileSearch.results) == 0:
		line(faint.Render("  nothing matches " + m.fileSearch.query))
	}

	visibleH := max(h-len(lines), 1)
	offset := 0
	if m.fileSearch.idx >= visibleH {
		offset = m.fileSearch.idx - visibleH + 1
	}
	end := min(offset+visibleH, len(m.fileSearch.results))

	for i := offset; i < end; i++ {
		lines = append(lines, m.renderSearchRow(m.fileSearch.results[i], w, i == m.fileSearch.idx))
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}

	return lines
}

// renderSearchRow draws one result: its name, then the directory holding it,
// since the same name in two places is exactly what a search turns up.
func (m MainPane) renderSearchRow(match device.FileMatch, w int, selected bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)
	nameStyle := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	dirStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	metaStyle := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)

	// The selected row inverts, matching the tree's own cursor.
	if selected {
		bg = lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent)
		nameStyle = bg.Bold(true)
		dirStyle = bg
		metaStyle = bg
	}

	name := match.Name
	if match.IsDir {
		name += "/"
	}

	meta := ""
	if !match.IsDir {
		meta = formatSize(match.Size)
	}

	// Truncate from the left: search results frequently share a long
	// prefix (an account name, a bundle id) and differ only at the end, so
	// the tail is the part worth keeping.
	nameMax := max(w/2, 8)
	nameStr := truncPathLeft(name, nameMax)

	used := 1 + lipgloss.Width(nameStr) + 1 + lipgloss.Width(meta) + 1
	dirStr := truncPathLeft(gopath.Dir(match.Path), max(w-used-1, 0))

	out := bg.Render(" ") + nameStyle.Render(nameStr) + bg.Render(" ") + dirStyle.Render(dirStr)
	if pad := w - lipgloss.Width(out) - lipgloss.Width(meta) - 1; pad > 0 {
		out += bg.Render(strings.Repeat(" ", pad))
	}

	return out + metaStyle.Render(meta) + bg.Render(" ")
}

// renderSearchPreview describes the highlighted result in the preview pane:
// the full path above all, since a result list is mostly a question of
// "which one of these is it".
func (m MainPane) renderSearchPreview(w, h int) []string {
	match := m.SelectedMatch()

	lines := []string{padBg(w)}
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	line := func(s string) {
		if pad := w - lipgloss.Width(s); pad > 0 {
			s += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, s)
	}

	if match == nil {
		line(" " + faint.Render("no match selected"))
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return lines
	}

	kind := "file"
	if match.IsDir {
		kind = "directory"
	}

	line(" " + faint.Render("match ") + lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).
		Render(truncPathLeft(match.Name, max(w-8, 8))))
	line(padBg(w))
	for _, f := range []kv{
		{"Type", kind},
		{"Size", formatSize(match.Size)},
	} {
		if f.k == "Size" && match.IsDir {
			continue
		}
		line(renderField(f.k, f.v, w))
	}
	line(padBg(w))
	line(" " + faint.Render("path"))
	for _, chunk := range wrapPath(match.Path, max(w-2, 8)) {
		line(" " + lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render(chunk))
	}
	line(padBg(w))
	line(" " + faint.Render("enter opens it · esc returns to the tree"))

	for len(lines) < h {
		lines = append(lines, padBg(w))
	}

	return lines
}

// wrapPath splits a path into width-sized chunks so a long one stays
// readable in a narrow pane.
func wrapPath(path string, w int) []string {
	if w <= 0 {
		return nil
	}
	var out []string
	for runes := []rune(path); len(runes) > 0; {
		n := min(w, len(runes))
		out = append(out, string(runes[:n]))
		runes = runes[n:]
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// renderFilesFilterBar returns the filter segment for the "[3] Files" label
// row: the query being typed, and — once it is in use — the key that takes
// the same query to the device.
func (m MainPane) renderFilesFilterBar() string {
	active := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	if m.fileSearch.active {
		label := "device search: " + m.fileSearch.query
		if m.fileSearch.running {
			label += " …"
		} else if n := len(m.fileSearch.results); n > 0 {
			label += " · " + itoa(n) + " found"
			if n >= device.DefaultSearchLimit {
				label += "+"
			}
		}
		return faint.Render("⌕ ") + dim.Render(label) + faint.Render("  esc to close")
	}

	cursor := ""
	if m.filesFiltering {
		cursor = active.Render("█")
	}

	if m.filesFilter == "" && !m.filesFiltering {
		return faint.Render("/") + " " + faint.Render("filter files…")
	}

	hint := faint.Render("  ctrl+f searches the device")

	return faint.Render("/") + " " + dim.Render(m.filesFilter) + cursor + hint
}

func itoa(n int) string { return strconv.Itoa(n) }
