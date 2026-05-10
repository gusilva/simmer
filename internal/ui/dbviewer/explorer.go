package dbviewer

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"simmer/internal/device"
	"simmer/internal/theme"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type explorerNodeKind int

const (
	nodeKindDB explorerNodeKind = iota
	nodeKindFolder
	nodeKindTable
	nodeKindView
	nodeKindIndex
	nodeKindCol
)

type explorerNode struct {
	kind      explorerNodeKind
	label     string
	meta      string
	isPK      bool
	isFK      bool
	loading   bool
	disabled  bool
	connColor color.Color
	children  []*explorerNode
	expanded  bool
}

type visibleItem struct {
	node   *explorerNode
	indent int
}

// Explorer is the left-panel tree navigator inside the database viewer modal.
type Explorer struct {
	roots        []*explorerNode
	cursor       int
	filterActive bool
	filterQuery  string
	rh           renderHelpers
	loading      bool
	nTables      int
}

func newExplorer() Explorer {
	return Explorer{rh: newRenderHelpers()}
}

// SetLoading marks the explorer as waiting for a table list response.
func (e *Explorer) SetLoading() {
	e.loading = true
	e.roots = nil
	e.cursor = 0
	e.nTables = 0
	e.filterActive = false
	e.filterQuery = ""
}

// SetTables rebuilds the explorer tree from a live SQLiteObjects result.
func (e *Explorer) SetTables(dbName string, objs device.SQLiteObjects) {
	e.loading = false
	e.nTables = len(objs.Tables)

	tableNodes := make([]*explorerNode, len(objs.Tables))
	for i, t := range objs.Tables {
		tableNodes[i] = &explorerNode{kind: nodeKindTable, label: t}
	}

	viewNodes := make([]*explorerNode, len(objs.Views))
	for i, v := range objs.Views {
		viewNodes[i] = &explorerNode{kind: nodeKindView, label: v}
	}

	indexNodes := make([]*explorerNode, len(objs.Indexes))
	for i, idx := range objs.Indexes {
		indexNodes[i] = &explorerNode{kind: nodeKindIndex, label: idx}
	}

	tablesFolder := &explorerNode{
		kind:     nodeKindFolder,
		label:    "Tables",
		meta:     fmt.Sprintf("%d", len(objs.Tables)),
		expanded: true,
		children: tableNodes,
	}
	viewsFolder := &explorerNode{
		kind:     nodeKindFolder,
		label:    "Views",
		meta:     fmt.Sprintf("%d", len(objs.Views)),
		children: viewNodes,
	}
	indexesFolder := &explorerNode{
		kind:     nodeKindFolder,
		label:    "Indexes",
		meta:     fmt.Sprintf("%d", len(objs.Indexes)),
		children: indexNodes,
	}

	dbNode := &explorerNode{
		kind:      nodeKindDB,
		label:     stripExt(dbName),
		meta:      "SQLite",
		connColor: theme.ColorAccent,
		expanded:  true,
		children:  []*explorerNode{tablesFolder, viewsFolder, indexesFolder},
	}

	e.roots = []*explorerNode{dbNode}
	e.cursor = 0
}

func (e Explorer) visibleItems() []visibleItem {
	var items []visibleItem
	var walk func([]*explorerNode, int)
	walk = func(nodes []*explorerNode, indent int) {
		for _, n := range nodes {
			items = append(items, visibleItem{n, indent})
			if n.expanded {
				walk(n.children, indent+1)
			}
		}
	}
	walk(e.roots, 0)
	return items
}

func nodeMatchesFilter(n *explorerNode, query string) bool {
	if strings.Contains(strings.ToLower(n.label), query) {
		return true
	}
	for _, child := range n.children {
		if nodeMatchesFilter(child, query) {
			return true
		}
	}
	return false
}

func (e Explorer) filteredVisibleItems() []visibleItem {
	if e.filterQuery == "" {
		return e.visibleItems()
	}
	query := strings.ToLower(e.filterQuery)
	var items []visibleItem
	var walk func([]*explorerNode, int)
	walk = func(nodes []*explorerNode, indent int) {
		for _, n := range nodes {
			if !nodeMatchesFilter(n, query) {
				continue
			}
			items = append(items, visibleItem{n, indent})
			if n.expanded {
				walk(n.children, indent+1)
			}
		}
	}
	walk(e.roots, 0)
	return items
}

// selectedNodeInfo returns the selected node and, for column nodes, the label
// of the nearest ancestor table node in the visible list.
func (e Explorer) selectedNodeInfo() (node *explorerNode, parentTable string) {
	items := e.filteredVisibleItems()
	if e.cursor >= len(items) {
		return nil, ""
	}
	node = items[e.cursor].node
	if node.kind == nodeKindCol {
		for i := e.cursor - 1; i >= 0; i-- {
			if items[i].node.kind == nodeKindTable {
				parentTable = items[i].node.label
				break
			}
		}
	}
	return node, parentTable
}

func (e Explorer) Update(msg tea.Msg) (Explorer, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return e, nil
	}

	if e.filterActive {
		switch k.String() {
		case "esc":
			e.filterActive = false
			e.filterQuery = ""
			e.cursor = 0
		case "enter":
			e.filterActive = false
		case "backspace", "ctrl+h":
			if e.filterQuery != "" {
				runes := []rune(e.filterQuery)
				e.filterQuery = string(runes[:len(runes)-1])
				e.cursor = 0
			}
		default:
			runes := []rune(k.String())
			if len(runes) == 1 && unicode.IsPrint(runes[0]) {
				e.filterQuery += string(runes[0])
				e.cursor = 0
			}
		}
		return e, nil
	}

	items := e.filteredVisibleItems()
	n := len(items)
	switch k.String() {
	case "/":
		e.filterActive = true
		e.filterQuery = ""
		e.cursor = 0
	case "up", "k":
		if e.cursor > 0 {
			e.cursor--
		}
	case "down", "j":
		if e.cursor < n-1 {
			e.cursor++
		}
	case "enter", "space":
		if e.cursor < n {
			node := items[e.cursor].node
			if len(node.children) > 0 {
				node.expanded = !node.expanded
				newN := len(e.filteredVisibleItems())
				if e.cursor >= newN {
					e.cursor = newN - 1
				}
			}
		}
	}
	return e, nil
}

// Rows returns height strings each exactly width visual cells wide.
//
// Layout:
//
//	row 0           panel header  "[e] Explorer    3 dbs"
//	row 1           separator     "──────────────────────"
//	row 2           group label   "  CONNECTIONS"
//	rows 3..h-2     scrollable tree items
//	row h-1         footer        "/ filter tables…"
func (e Explorer) Rows(width, height int) []string {
	rh := e.rh
	selBgStyle := lipgloss.NewStyle().Background(theme.ColorAccent)

	fillTo := func(s string, sel bool) string {
		need := width - lipgloss.Width(s)
		if need <= 0 {
			return s
		}
		if sel {
			return s + selBgStyle.Render(strings.Repeat(" ", need))
		}
		return s + rh.BlankN(need)
	}

	out := make([]string, height)
	for i := range out {
		out[i] = rh.BlankN(width)
	}
	if height == 0 || width == 0 {
		return out
	}

	keyBadgeS := lipgloss.NewStyle().Foreground(theme.ColorBg).Background(theme.ColorAccent2).Bold(true).Padding(0, 1)
	boldFgS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg).Bold(true)
	faintS := lipgloss.NewStyle().Foreground(theme.ColorFgFaint).Background(theme.ColorBg)
	dimS := lipgloss.NewStyle().Foreground(theme.ColorFgDim).Background(theme.ColorBg)
	fgS := lipgloss.NewStyle().Foreground(theme.ColorFg).Background(theme.ColorBg)
	warnS := lipgloss.NewStyle().Foreground(theme.ColorWarn).Background(theme.ColorBg)
	infoS := lipgloss.NewStyle().Foreground(theme.ColorInfo).Background(theme.ColorBg)
	accentBoldS := lipgloss.NewStyle().Foreground(theme.ColorAccent).Background(theme.ColorBg).Bold(true)
	selFgS := lipgloss.NewStyle().Foreground(theme.ColorBg).Background(theme.ColorAccent).Bold(true)
	selDimS := lipgloss.NewStyle().Foreground(theme.ColorBg).Background(theme.ColorAccent)

	// row 0: panel header
	{
		left := keyBadgeS.Render("e") + rh.BlankN(1) + boldFgS.Render("Explorer")
		var rightTxt string
		switch {
		case e.loading:
			rightTxt = "loading…"
		case len(e.roots) == 0:
			rightTxt = "no db"
		default:
			rightTxt = fmt.Sprintf("%d tables", e.nTables)
		}

		right := faintS.Render(rightTxt)
		gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
		out[0] = fillTo(left+rh.BlankN(gap)+right, false)
	}

	if height == 1 {
		return out
	}

	// last row: footer
	{
		slash := accentBoldS.Render("/")
		var txt string
		switch {
		case e.filterActive:
			query := fgS.Render(" " + e.filterQuery)
			cursor := selFgS.Render("▌")
			txt = query + cursor
		case e.filterQuery != "":
			txt = dimS.Render(" " + e.filterQuery)
		default:
			txt = faintS.Render(" filter tables…")
		}
		out[height-1] = fillTo(slash+txt, false)
	}

	if height <= 2 {
		return out
	}

	out[1] = rh.Sep(width)
	if height <= 3 {
		return out
	}

	if height >= 5 {
		out[height-2] = rh.Sep(width)
	}

	out[2] = fillTo(rh.BlankN(2)+faintS.Render("CONNECTIONS"), false)
	if height <= 4 {
		return out
	}

	itemRows := height - 5

	// Show loading / empty state instead of tree.
	if e.loading {
		out[3] = fillTo(rh.BlankN(2)+faintS.Render("loading tables…"), false)

		return out
	}

	if len(e.roots) == 0 {
		out[3] = fillTo(rh.BlankN(2)+faintS.Render("no database loaded"), false)

		return out
	}

	items := e.filteredVisibleItems()

	scroll := 0
	if e.cursor >= itemRows {
		scroll = e.cursor - itemRows + 1
	}

	for i := 0; i < itemRows; i++ {
		idx := scroll + i
		if idx >= len(items) {
			out[i+3] = rh.BlankN(width)
			continue
		}
		item := items[idx]
		node := item.node
		sel := idx == e.cursor
		ind := item.indent

		indent := strings.Repeat(" ", ind*2)

		var caretRune string
		isExpandable := len(node.children) > 0 || node.kind == nodeKindTable || node.kind == nodeKindView
		if isExpandable {
			if node.expanded {
				caretRune = "▾"
			} else {
				caretRune = "▸"
			}
		} else {
			caretRune = " "
		}
		var caret string
		if sel {
			caret = selFgS.Render(caretRune)
		} else {
			caret = faintS.Render(caretRune)
		}

		var iconRune string
		var iconStyle lipgloss.Style
		switch node.kind {
		case nodeKindDB:
			if node.disabled {
				iconRune = "○"
				iconStyle = faintS
			} else {
				iconRune = "●"
				iconStyle = lipgloss.NewStyle().Foreground(node.connColor).Background(theme.ColorBg)
			}
		case nodeKindFolder:
			iconRune = " "
			iconStyle = faintS
		case nodeKindTable:
			iconRune = "≡"
			iconStyle = lipgloss.NewStyle().Foreground(theme.ColorAccent2).Background(theme.ColorBg)
		case nodeKindView:
			iconRune = "◈"
			iconStyle = lipgloss.NewStyle().Foreground(theme.ColorPink).Background(theme.ColorBg)
		case nodeKindIndex:
			iconRune = "◇"
			iconStyle = infoS
		case nodeKindCol:
			iconRune = "·"
			iconStyle = faintS
		}
		var icon string
		if sel {
			icon = selFgS.Render(iconRune)
		} else {
			icon = iconStyle.Render(iconRune)
		}

		var meta string
		switch {
		case node.loading:
			if sel {
				meta = selDimS.Render("…")
			} else {
				meta = faintS.Render("…")
			}

		case node.meta != "":
			if sel {
				meta = selDimS.Render(node.meta)
			} else {
				meta = faintS.Render(node.meta)
			}

		}

		// fixed overhead: indent(ind*2) + caret(1) + sp(1) + icon(1) + sp(1) = ind*2+4
		// prefix overhead per key indicator: isPK adds "⚿"(1), isFK adds "→"(1)
		metaW := lipgloss.Width(meta)
		pkOverhead := 0
		if node.isPK {
			pkOverhead++ // "⚿" is 1 cell
		}

		if node.isFK {
			pkOverhead++ // "→" is 1 cell
		}

		labelBudget := max(width-ind*2-4-pkOverhead-metaW-1, 1)
		nodeLabel := truncateLabel(node.label, labelBudget)

		var label string
		switch {
		case node.isPK:
			var pkStyle, lblStyle lipgloss.Style
			if sel {
				pkStyle = selFgS
				lblStyle = selFgS
			} else {
				pkStyle = warnS
				lblStyle = fgS
			}
			prefix := pkStyle.Render("⚿")
			if node.isFK {
				if sel {
					prefix = selFgS.Render("⚿→")
				} else {
					prefix = warnS.Render("⚿") + infoS.Render("→")
				}
			}
			label = prefix + lblStyle.Render(nodeLabel)

		case node.isFK:
			var fkStyle, lblStyle lipgloss.Style
			if sel {
				fkStyle = selFgS
				lblStyle = selFgS
			} else {
				fkStyle = infoS
				lblStyle = fgS
			}
			label = fkStyle.Render("→") + lblStyle.Render(nodeLabel)

		default:
			var lblStyle lipgloss.Style
			switch {
			case sel:
				lblStyle = selFgS
			case node.disabled:
				lblStyle = dimS
			default:
				lblStyle = fgS
			}
			label = lblStyle.Render(nodeLabel)
		}

		sp := func(n int) string {
			if n <= 0 {
				return ""
			}
			if sel {
				return selBgStyle.Render(strings.Repeat(" ", n))
			}
			return rh.BlankN(n)
		}

		prefix := sp(len(indent)) + caret + sp(1) + icon + sp(1) + label
		prefixW := lipgloss.Width(prefix)
		gap := width - prefixW - metaW
		if gap < 1 {
			gap = 1
		}
		out[i+3] = fillTo(prefix+sp(gap)+meta, sel)
	}

	return out
}

// stripExt removes the last file extension from name (e.g. "app.db" → "app").
func stripExt(name string) string {
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		return name[:i]
	}

	return name
}

// truncateLabel clips s to budget visible cells, appending "…" if truncated.
func truncateLabel(s string, budget int) string {
	runes := []rune(s)
	if len(runes) <= budget {
		return s
	}

	if budget <= 1 {
		return "…"
	}

	return string(runes[:budget-1]) + "…"
}
