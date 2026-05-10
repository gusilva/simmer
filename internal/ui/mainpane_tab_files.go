package ui

import (
	"fmt"
	"strings"

	"simmer/internal/device"

	"charm.land/lipgloss/v2"
)

func (m MainPane) renderFiles(innerW, innerH int) string {
	if m.tree == nil {
		lines := m.renderTreePane(innerW, innerH)
		return strings.Join(lines, "\n")
	}

	treeW, _, previewW := filesLayout(innerW)

	treeLines := m.renderTreePane(treeW, innerH)
	previewLines := m.renderPreviewPane(previewW, innerH)

	sep := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Background(ColorBg).
		Render("│")

	rows := make([]string, 0, innerH)
	for i := range innerH {
		t := padBg(treeW)
		p := padBg(previewW)
		if i < len(treeLines) {
			t = treeLines[i]
		}
		if i < len(previewLines) {
			p = previewLines[i]
		}
		rows = append(rows, t+sep+p)
	}
	return strings.Join(rows, "\n")
}

// ── Files: tree pane ───────────────────────────────────────────────────

type treeRow struct {
	node  *device.FileNode
	depth int
}

func (m MainPane) flattenTree() []treeRow {
	var out []treeRow
	if m.tree == nil {
		return out
	}
	var walk func(n *device.FileNode, depth int)
	walk = func(n *device.FileNode, depth int) {
		out = append(out, treeRow{node: n, depth: depth})
		if !n.IsDir || !m.expanded[n.Path] {
			return
		}
		for i := range n.Children {
			walk(&n.Children[i], depth+1)
		}
	}
	for i := range m.tree.Children {
		walk(&m.tree.Children[i], 0)
	}
	return out
}

func (m MainPane) renderTreePane(w, h int) []string {
	lines := []string{m.renderCrumb(w), padBg(w)}

	if m.tree == nil && m.active != nil {
		hint := "  loading files…"
		rendered := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(hint)
		pad := w - lipgloss.Width(rendered)
		if pad > 0 {
			rendered += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, rendered)
		for len(lines) < h {
			lines = append(lines, padBg(w))
		}
		return lines
	}

	visibleH := max(h-len(lines), 1)
	rows := m.flattenTree()

	// Window the slice so the cursor is always visible.
	offset := 0
	if m.treeIdx >= visibleH {
		offset = m.treeIdx - visibleH + 1
	}
	end := min(offset+visibleH, len(rows))

	for i := offset; i < end; i++ {
		lines = append(lines, m.renderTreeRow(rows[i], w, i == m.treeIdx))
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	return lines
}

func (m MainPane) renderCrumb(w int) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	const lead = " "
	const rootPrefix = "~/"
	const sep = " · "

	plainName := m.active.Name
	plainPath := ""
	if m.tree != nil {
		plainPath = m.tree.Path
	}

	// Budget the plain text to fit `w` cells. Trim path first, then name.
	avail := w - len(lead) - len(rootPrefix)
	if avail < 1 {
		return bg.Render(strings.Repeat(" ", w))
	}
	if plainPath != "" {
		if budget := avail - lipgloss.Width(plainName) - len(sep); budget > 0 {
			plainPath = truncateName(plainPath, budget)
		} else {
			plainPath = ""
		}
	}
	if lipgloss.Width(plainName)+len(sep)+lipgloss.Width(plainPath) > avail {
		nameBudget := avail - len(sep) - lipgloss.Width(plainPath)
		if nameBudget < 1 {
			nameBudget = avail
			plainPath = ""
		}
		plainName = truncateName(plainName, nameBudget)
	}

	root := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render(rootPrefix)
	name := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(plainName)
	rest := ""
	if plainPath != "" {
		rest = lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render(sep + plainPath)
	}
	row := lead + root + name + rest
	if pad := w - lipgloss.Width(row); pad > 0 {
		row += bg.Render(strings.Repeat(" ", pad))
	}
	return row
}

func (m MainPane) renderTreeRow(r treeRow, w int, selected bool) string {
	bg := lipgloss.NewStyle().Background(ColorBg)

	indent := strings.Repeat("  ", r.depth)

	caretCh := " "
	if r.node.IsDir {
		if m.expanded[r.node.Path] {
			caretCh = "▾"
		} else {
			caretCh = "▸"
		}
	}

	meta := ""
	if !r.node.IsDir {
		meta = formatSize(r.node.Size)
	} else if len(r.node.Children) == 0 {
		meta = "—"
	}

	// Layout: " " + indent + caret(1) + " " + name + GAP + meta + " "
	prefixW := 1 + lipgloss.Width(indent) + 1 + 1
	metaW := lipgloss.Width(meta)
	const minGap = 1
	const trailW = 1
	nameMax := max(w-prefixW-metaW-trailW-minGap, 1)
	nameStr := truncateName(r.node.Name, nameMax)
	nameW := lipgloss.Width(nameStr)
	gap := max(w-prefixW-nameW-metaW-trailW, minGap)

	if selected {
		sel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
		return sel.Render(" " + indent + caretCh + " " + nameStr +
			strings.Repeat(" ", gap) + meta + " ")
	}

	nameC := ColorFg
	if r.node.IsDir {
		nameC = ColorInfo
	}
	caret := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(caretCh)
	name := lipgloss.NewStyle().Foreground(nameC).Background(ColorBg).Render(nameStr)
	metaStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(meta)

	left := " " + indent + caret + " " + name
	return left + bg.Render(strings.Repeat(" ", gap)) + metaStyled + bg.Render(" ")
}

// ── Files: preview pane ────────────────────────────────────────────────

func (m MainPane) renderPreviewPane(w, h int) []string {
	rows := m.flattenTree()
	var sel *device.FileNode
	if m.treeIdx >= 0 && m.treeIdx < len(rows) {
		sel = rows[m.treeIdx].node
	}

	bg := lipgloss.NewStyle().Background(ColorBg)
	var lines []string

	if sel == nil {
		lines = append(lines, " "+lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("preview"))
	} else {
		const labelText = "preview "
		nameBudget := w - 1 - len(labelText)
		nameStr := truncateName(sel.Name, nameBudget)
		labelL := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(labelText)
		labelN := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(nameStr)
		lines = append(lines, " "+labelL+labelN)
	}
	lines = append(lines, padBg(w))

	if sel != nil {
		for _, f := range previewFields(sel) {
			lines = append(lines, renderField(f.k, f.v, w))
		}
		lines = append(lines, padBg(w))
		lines = append(lines, " "+lipgloss.NewStyle().
			Foreground(ColorFgFaint).
			Background(ColorBg).
			Render("── content ──"))
		for _, body := range previewContent(sel) {
			lines = append(lines, " "+lipgloss.NewStyle().
				Foreground(ColorFgDim).
				Background(ColorBg).
				Render(body))
		}
		// SQLite viewer hint
		if !sel.IsDir && m.active != nil {
			lname := strings.ToLower(sel.Name)
			if strings.HasSuffix(lname, ".db") || strings.HasSuffix(lname, ".sqlite") || strings.HasSuffix(lname, ".sqlite3") {
				lines = append(lines, padBg(w))
				lines = append(lines, " "+lipgloss.NewStyle().
					Foreground(ColorAccent).
					Background(ColorBg).
					Render("Enter → SQLite viewer"))
			}
		}
	}

	for i, line := range lines {
		if pad := w - lipgloss.Width(line); pad > 0 {
			lines[i] = line + bg.Render(strings.Repeat(" ", pad))
		}
	}
	for len(lines) < h {
		lines = append(lines, padBg(w))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

type kv struct{ k, v string }

func previewFields(n *device.FileNode) []kv {
	if n.IsDir {
		return []kv{
			{"type", "directory"},
			{"items", fmt.Sprintf("%d", len(n.Children))},
			{"modified", n.Modified.Format("2006-01-02 15:04:05")},
			{"permissions", n.Permissions},
		}
	}
	return []kv{
		{"type", "file " + fileExtSuffix(n.Name)},
		{"size", formatSize(n.Size)},
		{"modified", n.Modified.Format("2006-01-02 15:04:05")},
		{"permissions", n.Permissions},
	}
}

func previewContent(n *device.FileNode) []string {
	if n.IsDir {
		return []string{fmt.Sprintf("(directory — %d entries)", len(n.Children))}
	}
	lname := strings.ToLower(n.Name)
	if strings.HasSuffix(lname, ".db") || strings.HasSuffix(lname, ".sqlite") || strings.HasSuffix(lname, ".sqlite3") {
		return []string{
			"SQLite database",
			formatSize(n.Size),
		}
	}
	if strings.HasSuffix(n.Name, ".log") || strings.HasSuffix(n.Name, ".json") {
		return []string{fmt.Sprintf("(text — %s)", formatSize(n.Size))}
	}
	return []string{fmt.Sprintf("(binary — %s)", formatSize(n.Size))}
}

func renderField(k, v string, w int) string {
	const keyW = 12
	const trailW = 1
	keyText := k
	if len(keyText) > keyW {
		keyText = keyText[:keyW]
	}
	keyText = padRight(keyText, keyW)

	valBudget := max(w-1-keyW-1-trailW, 1)
	if lipgloss.Width(v) > valBudget {
		v = truncateName(v, valBudget)
	}

	keyStyled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg).Render(keyText)
	valStyled := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg).Render(v)

	row := " " + keyStyled + " " + valStyled
	if pad := w - lipgloss.Width(row); pad > 0 {
		row += lipgloss.NewStyle().Background(ColorBg).Render(strings.Repeat(" ", pad))
	}
	return row
}

func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

func fileExtSuffix(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return "(" + name[i:] + ")"
	}
	return ""
}

func formatSize(b int64) string {
	switch {
	case b >= 1_000_000_000:
		return fmt.Sprintf("%.1f GB", float64(b)/1_000_000_000)
	case b >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	case b >= 1_000:
		return fmt.Sprintf("%d KB", b/1_000)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
