package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

// sandboxPane builds a Files panel over an app-storage tree: a synthetic
// root whose two children are the storage areas, which is the shape that
// makes search need more than one root.
func sandboxPane() MainPane {
	root := &device.FileNode{
		Name: "com.acme.app", IsDir: true, // a label for the app's storage, not a path
		Children: []device.FileNode{
			{Path: "/data/data/com.acme.app", Name: "internal", IsDir: true, Children: []device.FileNode{
				{Path: "/data/data/com.acme.app/databases", Name: "databases", IsDir: true, Children: []device.FileNode{
					{Path: "/data/data/com.acme.app/databases/rooms.db", Name: "rooms.db", Size: 4096},
					{Path: "/data/data/com.acme.app/databases/notes.txt", Name: "notes.txt", Size: 10},
				}},
				{Path: "/data/data/com.acme.app/cache", Name: "cache", IsDir: true, Children: []device.FileNode{
					{Path: "/data/data/com.acme.app/cache/tmp.bin", Name: "tmp.bin", Size: 1},
				}},
			}},
			{Path: "/storage/emulated/0/Android/data/com.acme.app", Name: "external", IsDir: true},
		},
	}

	m := newPaneWithDevice()
	m.SetSize(120, 30)
	m.SetTree(root)
	m.panel = panelFiles
	return m
}

// expandAll opens every directory so the filter has rows to work on.
func expandAll(m *MainPane, n *device.FileNode) {
	if !n.IsDir {
		return
	}
	m.expanded[n.Path] = true
	for i := range n.Children {
		expandAll(m, &n.Children[i])
	}
}

func rowNames(rows []treeRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.node.Name)
	}
	return out
}

func TestFilterKeepsMatchesInContext(t *testing.T) {
	m := sandboxPane()
	expandAll(&m, m.tree)

	m.filesFilter = "rooms"
	names := rowNames(m.visibleTreeRows())

	// The match is kept, and so are the directories leading to it — a
	// match shown without its place in the tree says nothing about where
	// the file is.
	for _, want := range []string{"rooms.db", "databases", "internal"} {
		if !contains(names, want) {
			t.Errorf("rows %v are missing %q", names, want)
		}
	}
	// Unrelated branches are gone.
	for _, unwanted := range []string{"notes.txt", "tmp.bin", "cache"} {
		if contains(names, unwanted) {
			t.Errorf("rows %v should not contain %q", names, unwanted)
		}
	}
}

func TestFilterMatchesTheSameWayTheDeviceSearchDoes(t *testing.T) {
	// ctrl+f hands the typed text to the device search, so the two must
	// agree: a fuzzy filter would accept "rms" and then find nothing.
	m := sandboxPane()
	expandAll(&m, m.tree)

	for _, q := range []string{"rooms", "ROOMS", "ms.db"} {
		m.filesFilter = q
		if !contains(rowNames(m.visibleTreeRows()), "rooms.db") {
			t.Errorf("substring %q did not match rooms.db", q)
		}
	}

	for _, q := range []string{"rms", "r.db", "zzz"} {
		m.filesFilter = q
		if contains(rowNames(m.visibleTreeRows()), "rooms.db") {
			t.Errorf("query %q matched rooms.db, but the device search would not", q)
		}
	}
}

func TestFilterKeysTypeAndClear(t *testing.T) {
	m := sandboxPane()
	expandAll(&m, m.tree)

	m, _ = m.Update(tea.KeyPressMsg{Text: "/"})
	if !m.FilterActive() {
		t.Fatal("/ should start the filter")
	}
	for _, r := range "rooms" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(r)})
	}
	if m.filesFilter != "rooms" {
		t.Fatalf("filter = %q", m.filesFilter)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.filesFilter != "room" {
		t.Errorf("backspace left %q", m.filesFilter)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.FilterActive() {
		t.Error("enter should stop capturing keys")
	}
	if m.filesFilter != "room" {
		t.Error("enter should keep the filter applied")
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "/"})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.filesFilter != "" || m.FilterActive() {
		t.Error("esc should clear the filter")
	}
}

func TestSearchRootsCoverEveryStorageArea(t *testing.T) {
	m := sandboxPane()

	roots := m.SearchRoots()
	want := []string{"/data/data/com.acme.app", "/storage/emulated/0/Android/data/com.acme.app"}
	if len(roots) != len(want) {
		t.Fatalf("roots = %v, want both areas", roots)
	}
	for i := range want {
		if roots[i] != want[i] {
			t.Errorf("roots = %v, want %v", roots, want)
		}
	}
}

func TestSearchRootsOfAPlainTree(t *testing.T) {
	m := newPaneWithDevice()
	m.SetTree(&device.FileNode{Path: "/storage/emulated/0", Name: "0", IsDir: true,
		Children: []device.FileNode{{Path: "/storage/emulated/0/Download", Name: "Download", IsDir: true}},
	})

	roots := m.SearchRoots()
	if len(roots) != 1 || roots[0] != "/storage/emulated/0" {
		t.Errorf("roots = %v, want just the tree root", roots)
	}
}

func TestCtrlFEscalatesTheTypedFilter(t *testing.T) {
	m := sandboxPane()

	m, _ = m.Update(tea.KeyPressMsg{Text: "/"})
	for _, r := range "rooms" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(r)})
	}

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+f should start a device search")
	}

	msg, ok := cmd().(RequestFileSearchMsg)
	if !ok {
		t.Fatalf("got %#v, want RequestFileSearchMsg", cmd())
	}
	if msg.Query != "rooms" {
		t.Errorf("query = %q, want the text already typed", msg.Query)
	}
	if len(msg.Roots) != 2 {
		t.Errorf("roots = %v, want both storage areas", msg.Roots)
	}
	if !m.SearchActive() {
		t.Error("the pane should show the search while it runs")
	}
}

func TestCtrlFWithoutAQueryDoesNothing(t *testing.T) {
	m := sandboxPane()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if cmd != nil {
		t.Error("an empty query would match the whole device")
	}
}

func searchingPane(t *testing.T) MainPane {
	t.Helper()
	m := sandboxPane()
	m.SetFileSearchResults("rooms", []device.FileMatch{
		{Name: "rooms.db", Path: "/data/data/com.acme.app/databases/rooms.db", Size: 4096},
		{Name: "rooms", Path: "/storage/emulated/0/Android/data/com.acme.app/rooms", IsDir: true},
		{Name: "rooms.txt", Path: "/storage/emulated/0/Android/data/com.acme.app/notes/rooms.txt"},
	})
	return m
}

func TestSearchResultNavigationClamps(t *testing.T) {
	m := searchingPane(t)

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.fileSearch.idx != 0 {
		t.Errorf("idx = %d at the top", m.fileSearch.idx)
	}

	for range 5 {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.fileSearch.idx != 2 {
		t.Errorf("idx = %d, want the last result", m.fileSearch.idx)
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "g"})
	if m.fileSearch.idx != 0 {
		t.Errorf("g should jump to the top, idx = %d", m.fileSearch.idx)
	}
}

func TestActivatingADatabaseResultOpensTheViewer(t *testing.T) {
	m := searchingPane(t)

	cmd := m.ActivateSearchResult()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg, ok := cmd().(ShowSQLiteViewerMsg)
	if !ok {
		t.Fatalf("got %#v, want ShowSQLiteViewerMsg", cmd())
	}
	if msg.DBPath != "/data/data/com.acme.app/databases/rooms.db" {
		t.Errorf("path = %q", msg.DBPath)
	}
}

func TestActivatingADirectoryResultRerootsTheTree(t *testing.T) {
	m := searchingPane(t)
	m.fileSearch.idx = 1

	msg, ok := m.ActivateSearchResult()().(RequestFileTreeAtMsg)
	if !ok {
		t.Fatal("want RequestFileTreeAtMsg for a directory")
	}
	if msg.Path != "/storage/emulated/0/Android/data/com.acme.app/rooms" {
		t.Errorf("path = %q", msg.Path)
	}
}

func TestActivatingAPlainFileRerootsAtItsDirectory(t *testing.T) {
	// A file that cannot be opened still has somewhere worth going: the
	// directory holding it, where it can be previewed.
	m := searchingPane(t)
	m.fileSearch.idx = 2

	msg, ok := m.ActivateSearchResult()().(RequestFileTreeAtMsg)
	if !ok {
		t.Fatal("want RequestFileTreeAtMsg for a plain file")
	}
	if msg.Path != "/storage/emulated/0/Android/data/com.acme.app/notes" {
		t.Errorf("path = %q, want the parent directory", msg.Path)
	}
}

func TestEscLeavesSearchAndRestoresTheTree(t *testing.T) {
	m := searchingPane(t)
	expandAll(&m, m.tree)

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.SearchActive() {
		t.Error("esc should close the search")
	}
	if len(m.visibleTreeRows()) == 0 {
		t.Error("the tree should be back")
	}
}

func TestSearchViewReportsCountAndEmptiness(t *testing.T) {
	m := searchingPane(t)
	if bar := m.renderFilesFilterBar(); !strings.Contains(bar, "3 found") {
		t.Errorf("filter bar %q does not report the count", bar)
	}

	m.SetFileSearchResults("zzz", nil)
	pane := strings.Join(m.renderSearchPane(80, 10), "\n")
	if !strings.Contains(pane, "nothing matches zzz") {
		t.Errorf("empty search does not say so: %q", pane)
	}
}

func TestSearchViewReportsFailure(t *testing.T) {
	m := sandboxPane()
	m.SetFileSearchError("rooms", "permission denied")

	pane := strings.Join(m.renderSearchPane(80, 10), "\n")
	if !strings.Contains(pane, "permission denied") {
		t.Errorf("failure not shown: %q", pane)
	}
}

func TestFilterBarAdvertisesTheDeviceSearch(t *testing.T) {
	// A filter that comes up empty must not be mistaken for a file that is
	// not on the device, so the escalation is named next to the input.
	m := sandboxPane()
	m.filesFiltering = true
	m.filesFilter = "rooms"

	if bar := m.renderFilesFilterBar(); !strings.Contains(bar, "ctrl+f") {
		t.Errorf("filter bar %q does not mention the device search", bar)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func TestLeavingSearchClearsTheFilter(t *testing.T) {
	// The query was handed to the device search, so leaving it applied
	// would narrow the tree by a term no longer shown anywhere — which
	// reads as a tree that refuses to open.
	m := sandboxPane()
	expandAll(&m, m.tree)

	m, _ = m.Update(tea.KeyPressMsg{Text: "/"})
	for _, r := range "rooms" {
		m, _ = m.Update(tea.KeyPressMsg{Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m.SetFileSearchResults("rooms", nil)

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.filesFilter != "" {
		t.Errorf("filter = %q after leaving the search, want it cleared", m.filesFilter)
	}
	names := rowNames(m.visibleTreeRows())
	for _, want := range []string{"cache", "notes.txt"} {
		if !contains(names, want) {
			t.Errorf("rows %v are still narrowed; the whole tree should be back", names)
		}
	}
}

func TestPickingAResultClearsTheFilterToo(t *testing.T) {
	m := searchingPane(t)
	m.filesFilter = "rooms"

	m.ExitFileSearch() // what the parent does when a result is picked

	if m.filesFilter != "" || m.FilterActive() {
		t.Error("picking a result should leave both the search and the filter")
	}
}

func TestFilteredDirectoryStillOpens(t *testing.T) {
	// The reported bug: a directory whose own name matched showed an open
	// caret and a preview saying two entries, but no child rows, because
	// the children did not match the query themselves.
	root := &device.FileNode{
		Path: "/Android/data", Name: "data", IsDir: true,
		Children: []device.FileNode{
			{Path: "/Android/data/com.osapiens.maintenanceos.qa", Name: "com.osapiens.maintenanceos.qa", IsDir: true, Children: []device.FileNode{
				{Path: "/Android/data/com.osapiens.maintenanceos.qa/cache", Name: "cache", IsDir: true},
				{Path: "/Android/data/com.osapiens.maintenanceos.qa/files", Name: "files", IsDir: true},
			}},
			{Path: "/Android/data/com.other.app", Name: "com.other.app", IsDir: true, Children: []device.FileNode{
				{Path: "/Android/data/com.other.app/cache", Name: "cache", IsDir: true},
			}},
		},
	}

	m := newPaneWithDevice()
	m.SetSize(120, 30)
	m.SetTree(root)
	expandAll(&m, root)
	m.filesFilter = "osapiens"

	names := rowNames(m.visibleTreeRows())

	// The matched directory opens onto its own contents…
	for _, want := range []string{"com.osapiens.maintenanceos.qa", "cache", "files"} {
		if !contains(names, want) {
			t.Errorf("rows %v are missing %q", names, want)
		}
	}
	// …while the unrelated branch stays out, including its own "cache".
	if contains(names, "com.other.app") {
		t.Errorf("rows %v should not contain the unrelated app", names)
	}
	if n := countOf(names, "cache"); n != 1 {
		t.Errorf("rows %v contain %d caches, want only the matched app's", names, n)
	}
}

func TestFilterKeepsDeepDescendantsOfAMatch(t *testing.T) {
	root := &device.FileNode{
		Path: "/a", Name: "a", IsDir: true,
		Children: []device.FileNode{{
			Path: "/a/target", Name: "target", IsDir: true,
			Children: []device.FileNode{{
				Path: "/a/target/mid", Name: "mid", IsDir: true,
				Children: []device.FileNode{
					{Path: "/a/target/mid/leaf.txt", Name: "leaf.txt"},
				},
			}},
		}},
	}

	m := newPaneWithDevice()
	m.SetTree(root)
	expandAll(&m, root)
	m.filesFilter = "target"

	names := rowNames(m.visibleTreeRows())
	for _, want := range []string{"target", "mid", "leaf.txt"} {
		if !contains(names, want) {
			t.Errorf("rows %v are missing %q: a match opens onto everything below it", names, want)
		}
	}
}

func TestTreeParents(t *testing.T) {
	rows := []treeRow{
		{node: &device.FileNode{Name: "a"}, depth: 0},
		{node: &device.FileNode{Name: "a1"}, depth: 1},
		{node: &device.FileNode{Name: "a1x"}, depth: 2},
		{node: &device.FileNode{Name: "a2"}, depth: 1},
		{node: &device.FileNode{Name: "b"}, depth: 0},
		{node: &device.FileNode{Name: "b1"}, depth: 1},
	}

	got := treeParents(rows)
	want := []int{-1, 0, 1, 0, -1, 4}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parent of %q = %d, want %d", rows[i].node.Name, got[i], want[i])
		}
	}
}

func countOf(haystack []string, needle string) int {
	n := 0
	for _, h := range haystack {
		if h == needle {
			n++
		}
	}
	return n
}
