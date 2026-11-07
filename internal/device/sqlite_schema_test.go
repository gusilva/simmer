package device

import (
	"reflect"
	"testing"
)

func TestParseSchemaColumns(t *testing.T) {
	tests := []struct {
		name    string
		rows    [][]string
		want    []SchemaTable
		wantErr bool
	}{
		{"empty", nil, nil, false},
		{"header only", [][]string{{"type", "tbl", "col"}}, nil, false},
		{
			"tables and a view, in order",
			[][]string{
				{"type", "tbl", "col"},
				{"table", "orders", "id"},
				{"table", "orders", "user_id"},
				{"view", "recent", "id"},
				{"table", "users", "id"},
				{"table", "users", "name"},
			},
			[]SchemaTable{
				{Name: "orders", Columns: []string{"id", "user_id"}},
				{Name: "recent", IsView: true, Columns: []string{"id"}},
				{Name: "users", Columns: []string{"id", "name"}},
			},
			false,
		},
		{
			"columns found by header name",
			[][]string{{"col", "tbl", "type"}, {"id", "t", "table"}},
			[]SchemaTable{{Name: "t", Columns: []string{"id"}}},
			false,
		},
		{"short rows skipped", [][]string{{"type", "tbl", "col"}, {"table", "t"}}, nil, false},
		{"unexpected header", [][]string{{"name"}, {"t"}}, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSchemaColumns(tc.rows)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// TestSchemaColumnsQuery_LocalSQLite runs the real query against a database
// built with the host's sqlite3, the route the simulator and every copied
// database take.
func TestSchemaColumnsQuery_LocalSQLite(t *testing.T) {
	db := t.TempDir() + "/t.db"
	setup := "CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT);" +
		"CREATE TABLE \"odd name\"(\"a b\" TEXT);" +
		"CREATE VIEW v AS SELECT name FROM users;" +
		"CREATE INDEX i ON users(name);"
	if _, err := querySQLiteLocal(t.Context(), nil, db, setup); err != nil {
		t.Skipf("host sqlite3 unavailable: %v", err)
	}
	got, err := ListSchemaColumns(t.Context(), nil, Device{Platform: PlatformIOS, Kind: KindVirtual}, "", db)
	if err != nil {
		t.Fatalf("ListSchemaColumns: %v", err)
	}
	want := []SchemaTable{
		{Name: "odd name", Columns: []string{"a b"}},
		{Name: "users", Columns: []string{"id", "name"}},
		{Name: "v", IsView: true, Columns: []string{"name"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}
