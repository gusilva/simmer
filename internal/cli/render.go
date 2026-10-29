package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// writef writes formatted output and deliberately ignores the write error:
// a verb cannot act on a closed stdout, and the exit code already reports
// whether the operation itself succeeded.
func writef(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

// table writes rows as columns padded to their widest cell, tab-free and
// stable, so output stays readable by eye and cuttable by script.
func table(w io.Writer, header []string, rows [][]string) {
	all := make([][]string, 0, len(rows)+1)
	if len(header) > 0 {
		all = append(all, header)
	}
	all = append(all, rows...)

	widths := columnWidths(all)
	for _, row := range all {
		var b strings.Builder
		for i, cell := range row {
			if i == len(row)-1 {
				b.WriteString(cell)
				break
			}
			b.WriteString(pad(cell, widths[i]))
			b.WriteString("  ")
		}
		writef(w, "%s\n", strings.TrimRight(b.String(), " "))
	}
}

// columnWidths returns the widest cell per column.
func columnWidths(rows [][]string) []int {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			for len(widths) <= i {
				widths = append(widths, 0)
			}
			if n := len([]rune(cell)); n > widths[i] {
				widths[i] = n
			}
		}
	}
	return widths
}

func pad(s string, n int) string {
	if d := n - len([]rune(s)); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// emitJSON writes v as indented JSON with a trailing newline.
func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
