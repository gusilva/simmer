package ui

import (
	"image/color"
	"strings"
	"unicode"

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
	disabled  bool
	connColor color.Color // used by nodeKindDB
	children  []*explorerNode
	expanded  bool
}

type visibleItem struct {
	node   *explorerNode
	indent int
}

// DBExplorer is the left-panel tree navigator inside the database viewer modal.
type DBExplorer struct {
	roots        []*explorerNode
	cursor       int
	filterActive bool
	filterQuery  string
}

func newDBExplorer() DBExplorer {
	devices := &explorerNode{
		kind: nodeKindTable, label: "devices", meta: "21", expanded: true,
		children: []*explorerNode{
			{kind: nodeKindCol, label: "udid", meta: "TEXT", isPK: true, disabled: true},
			{kind: nodeKindCol, label: "name", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "os", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "state", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "family", meta: "TEXT", disabled: true},
			{kind: nodeKindCol, label: "booted_at", meta: "DATETIME", disabled: true},
			{kind: nodeKindCol, label: "cpu_pct", meta: "REAL", disabled: true},
			{kind: nodeKindCol, label: "… 4 more", disabled: true},
		},
	}

	simctl := &explorerNode{
		kind: nodeKindDB, label: "simctl.db", meta: "SQLite",
		connColor: ColorOrange, expanded: true,
		children: []*explorerNode{
			{
				kind: nodeKindFolder, label: "Tables", meta: "8", expanded: true,
				children: []*explorerNode{
					{kind: nodeKindTable, label: "apps", meta: "142"},
					devices,
					{kind: nodeKindTable, label: "device_runtimes", meta: "38"},
					{kind: nodeKindTable, label: "install_logs", meta: "1.2k"},
					{kind: nodeKindTable, label: "processes", meta: "847"},
					{kind: nodeKindTable, label: "preferences", meta: "93"},
					{kind: nodeKindTable, label: "media_assets", meta: "412"},
					{kind: nodeKindTable, label: "screenshots", meta: "28"},
				},
			},
			{kind: nodeKindFolder, label: "Views", meta: "3"},
			{kind: nodeKindFolder, label: "Indexes", meta: "14"},
			{kind: nodeKindFolder, label: "Triggers", meta: "2"},
			{kind: nodeKindFolder, label: "Sequences", meta: "n/a", disabled: true},
		},
	}

	return DBExplorer{
		roots: []*explorerNode{
			simctl,
			{kind: nodeKindDB, label: "logs.db", meta: "SQLite", connColor: ColorInfo},
			{kind: nodeKindDB, label: "analytics.duckdb", meta: "offline", disabled: true, connColor: ColorFgFaint},
		},
		cursor: 3, // "devices" is the 4th visible item (0-indexed)
	}
}

func (e DBExplorer) visibleItems() []visibleItem {
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

// nodeMatchesFilter reports whether n or any descendant label contains query.
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

// filteredVisibleItems returns the visible tree filtered by filterQuery.
// Parent nodes are kept when any descendant matches.
func (e DBExplorer) filteredVisibleItems() []visibleItem {
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

func (e DBExplorer) Update(msg tea.Msg) (DBExplorer, tea.Cmd) {
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
	case "enter", " ":
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
func (e DBExplorer) Rows(width, height int) []string {
	bgS := lipgloss.NewStyle().Background(ColorBg)
	blankN := func(n int) string {
		if n <= 0 {
			return ""
		}
		return bgS.Render(strings.Repeat(" ", n))
	}
	fillTo := func(s string, sel bool) string {
		need := width - lipgloss.Width(s)
		if need <= 0 {
			return s
		}
		if sel {
			return s + lipgloss.NewStyle().Background(ColorAccent).Render(strings.Repeat(" ", need))
		}
		return s + blankN(need)
	}

	out := make([]string, height)
	for i := range out {
		out[i] = blankN(width)
	}
	if height == 0 || width == 0 {
		return out
	}

	// shared styles
	keyBadgeS := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent2).Bold(true).Padding(0, 1)
	boldFgS := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Bold(true)
	sepS := lipgloss.NewStyle().Foreground(ColorBorder).Background(ColorBg)
	faintS := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	dimS := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	fgS := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	warnS := lipgloss.NewStyle().Foreground(ColorWarn).Background(ColorBg)
	infoS := lipgloss.NewStyle().Foreground(ColorInfo).Background(ColorBg)
	accentBoldS := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	selFgS := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
	selDimS := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent)
	selBgS := lipgloss.NewStyle().Background(ColorAccent)

	// row 0: panel header
	{
		left := keyBadgeS.Render("e") + blankN(1) + boldFgS.Render("Explorer")
		right := faintS.Render("3 dbs")
		gap := width - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		out[0] = fillTo(left+blankN(gap)+right, false)
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

	// row 1: top separator
	out[1] = sepS.Render(strings.Repeat("─", width))
	if height <= 3 {
		return out
	}

	// second-to-last row: footer separator (only when there is room above the group label)
	if height >= 5 {
		out[height-2] = sepS.Render(strings.Repeat("─", width))
	}

	// row 2: pinned group label
	out[2] = fillTo(blankN(2)+faintS.Render("CONNECTIONS"), false)
	if height <= 4 {
		return out
	}

	// rows 3..height-3: scrollable tree items
	itemRows := height - 5
	items := e.filteredVisibleItems()

	// compute scroll to keep cursor visible (cursor always at or above last visible row)
	scroll := 0
	if e.cursor >= itemRows {
		scroll = e.cursor - itemRows + 1
	}

	for i := 0; i < itemRows; i++ {
		idx := scroll + i
		if idx >= len(items) {
			out[i+3] = blankN(width)
			continue
		}
		item := items[idx]
		node := item.node
		sel := idx == e.cursor
		ind := item.indent

		indent := strings.Repeat(" ", ind*2)

		// caret
		var caretRune string
		if len(node.children) > 0 {
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

		// icon
		var iconRune string
		var iconStyle lipgloss.Style
		switch node.kind {
		case nodeKindDB:
			if node.disabled {
				iconRune = "○"
				iconStyle = faintS
			} else {
				iconRune = "●"
				iconStyle = lipgloss.NewStyle().Foreground(node.connColor).Background(ColorBg)
			}
		case nodeKindFolder:
			iconRune = " "
			iconStyle = faintS
		case nodeKindTable:
			iconRune = "≡"
			iconStyle = lipgloss.NewStyle().Foreground(ColorAccent2).Background(ColorBg)
		case nodeKindView:
			iconRune = "◈"
			iconStyle = lipgloss.NewStyle().Foreground(ColorPink).Background(ColorBg)
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

		// label (with optional PK indicator for primary-key columns)
		var label string
		if node.isPK {
			var pkStyle, lblStyle lipgloss.Style
			if sel {
				pkStyle = selFgS
				lblStyle = selFgS
			} else {
				pkStyle = warnS
				lblStyle = fgS
			}
			label = pkStyle.Render("⚿") + lblStyle.Render(node.label)
		} else {
			var lblStyle lipgloss.Style
			switch {
			case sel:
				lblStyle = selFgS
			case node.disabled:
				lblStyle = dimS
			default:
				lblStyle = fgS
			}
			label = lblStyle.Render(node.label)
		}

		// meta (right-aligned)
		var meta string
		if node.meta != "" {
			if sel {
				meta = selDimS.Render(node.meta)
			} else {
				meta = faintS.Render(node.meta)
			}
		}

		// spacer uses selection background when row is selected so the full row fills.
		sp := func(n int) string {
			if n <= 0 {
				return ""
			}
			if sel {
				return selBgS.Render(strings.Repeat(" ", n))
			}
			return blankN(n)
		}

		// assemble: indent + caret + " " + icon + " " + label ... meta
		prefix := sp(len(indent)) + caret + sp(1) + icon + sp(1) + label
		prefixW := lipgloss.Width(prefix)
		metaW := lipgloss.Width(meta)
		gap := width - prefixW - metaW
		if gap < 1 {
			gap = 1
		}
		out[i+3] = fillTo(prefix+sp(gap)+meta, sel)
	}

	return out
}
