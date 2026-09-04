package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"sqvue/internal/db"
)

func TestDriverBrowsesAndQueriesSQLite(t *testing.T) {
	ctx := context.Background()
	driver := New()
	if err := driver.Connect(ctx, db.ConnectConfig{DSN: filepath.Join(t.TempDir(), "sqvue.db")}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = driver.Close() })

	for _, statement := range []string{
		`create table categories (
			id integer primary key,
			parent_id integer references categories(id),
			name text not null,
			note text default 'none'
		)`,
		`insert into categories (name) values ('books'), ('games'), ('music')`,
		`create view category_names as select name from categories`,
	} {
		if _, err := driver.Query(ctx, db.Query{SQL: statement}); err != nil {
			t.Fatalf("Query(%q) error = %v", statement, err)
		}
	}

	schemas, err := driver.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas() error = %v", err)
	}
	if len(schemas) == 0 || schemas[0].Name != "main" {
		t.Fatalf("schemas = %#v, want main", schemas)
	}

	tables, err := driver.ListTables(ctx, "main")
	if err != nil {
		t.Fatalf("ListTables() error = %v", err)
	}
	if len(tables) != 2 || tables[0] != (db.Table{Schema: "main", Name: "categories", Type: "table"}) || tables[1] != (db.Table{Schema: "main", Name: "category_names", Type: "view"}) {
		t.Fatalf("tables = %#v", tables)
	}

	info, err := driver.DescribeTable(ctx, "main", "categories")
	if err != nil {
		t.Fatalf("DescribeTable() error = %v", err)
	}
	if len(info.Columns) != 4 || !info.Columns[0].IsPrimary || info.Columns[0].Nullable || !info.Columns[1].Nullable || info.Columns[1].ForeignKey == nil || *info.Columns[1].ForeignKey != (db.ForeignKey{Schema: "main", Table: "categories", Column: "id"}) || info.Columns[2].Nullable || info.Columns[3].Default == nil || *info.Columns[3].Default != "'none'" {
		t.Fatalf("columns = %#v", info.Columns)
	}

	columns, rows, err := driver.Rows(ctx, tables[0], 2, 0)
	if err != nil {
		t.Fatalf("Rows() error = %v", err)
	}

	if len(columns) != 4 || len(rows) != 2 || rows[0][2] != "books" || rows[1][2] != "games" {
		t.Fatalf("Rows() = columns %#v, rows %#v", columns, rows)
	}

	columns, rows, err = driver.BrowseRows(ctx, db.BrowseRequest{Table: tables[0], Limit: 2, Filters: []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "2"}}})
	if err != nil || len(columns) != 4 || len(rows) != 1 || rows[0][2] != "games" {
		t.Fatalf("BrowseRows() = columns %#v, rows %#v, error %v", columns, rows, err)
	}

	count, err := driver.CountRows(ctx, tables[0])
	if err != nil || count != 3 {
		t.Fatalf("CountRows() = %d, %v; want 3, nil", count, err)
	}

	result, err := driver.Query(ctx, db.Query{SQL: "select name from categories order by id"})
	if err != nil {
		t.Fatalf("select query error = %v", err)
	}
	if len(result.Columns) != 1 || len(result.Rows) != 3 || result.Rows[2][0] != "music" {
		t.Fatalf("select result = %#v", result)
	}

	result, err = driver.Query(ctx, db.Query{SQL: "update categories set note = 'featured' where id = 1"})
	if err != nil || result.RowsAffected != 1 {
		t.Fatalf("update result = %#v, error = %v", result, err)
	}
}

func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent(`a"b`); got != `"a""b"` {
		t.Fatalf("quoteIdent() = %q", got)
	}
}

func TestDriverRejectsOperationsBeforeConnect(t *testing.T) {
	driver := New()
	if _, err := driver.Query(context.Background(), db.Query{SQL: "select 1"}); err == nil {
		t.Fatal("Query() succeeded before Connect()")
	}
	if _, err := driver.CountRows(context.Background(), db.Table{Schema: "main", Name: "items"}); err == nil {
		t.Fatal("CountRows() succeeded before Connect()")
	}
}
