package device

import (
	"context"
	"fmt"
	"strings"

	"simmer/internal/logging"
)

// SchemaTable is a table or view with its column names, in declared order.
type SchemaTable struct {
	Name    string
	IsView  bool
	Columns []string
}

// schemaColumnsQuery lists every table and view with its columns in a single
// round trip — on Android each query is an adb call, and may mean copying the
// database off the device. The table-valued pragma needs SQLite 3.16 or later.
const schemaColumnsQuery = "SELECT m.type AS type, m.name AS tbl, p.name AS col" +
	" FROM sqlite_master AS m JOIN pragma_table_info(m.name) AS p" +
	" WHERE m.type IN ('table','view') AND m.name NOT LIKE 'sqlite_%'" +
	" ORDER BY m.name, p.cid;"

// ListSchemaColumns returns every table and view in the database with its
// columns. It ignores any configured tables query: completion needs the real
// identifiers, not display labels.
func ListSchemaColumns(ctx context.Context, logger *logging.Logger, dev Device, packageID, dbPath string) ([]SchemaTable, error) {
	rows, err := QuerySQLite(ctx, logger, dev, packageID, dbPath, schemaColumnsQuery)
	if err != nil {
		return nil, fmt.Errorf("schema columns: %w", err)
	}
	return parseSchemaColumns(rows)
}

// parseSchemaColumns folds the type/tbl/col rows of schemaColumnsQuery, header
// first, into one SchemaTable per table, keeping the query's order.
func parseSchemaColumns(rows [][]string) ([]SchemaTable, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	typeIdx, tblIdx, colIdx := -1, -1, -1
	for i, h := range rows[0] {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "type":
			typeIdx = i
		case "tbl":
			tblIdx = i
		case "col":
			colIdx = i
		}
	}
	if typeIdx < 0 || tblIdx < 0 || colIdx < 0 {
		return nil, fmt.Errorf("schema columns: unexpected header %v", rows[0])
	}
	width := max(typeIdx, tblIdx, colIdx) + 1

	var out []SchemaTable
	for _, row := range rows[1:] {
		if len(row) < width {
			continue
		}
		name := row[tblIdx]
		if n := len(out); n == 0 || out[n-1].Name != name {
			out = append(out, SchemaTable{Name: name, IsView: row[typeIdx] == "view"})
		}
		last := &out[len(out)-1]
		last.Columns = append(last.Columns, row[colIdx])
	}
	return out, nil
}
