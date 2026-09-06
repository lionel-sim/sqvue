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

	streamColumns, stream, err := driver.OpenTableRowStream(ctx, db.TableRowStreamRequest{Table: tables[0]})
	if err != nil {
		t.Fatalf("OpenTableRowStream() error = %v", err)
	}
	streamRows, exhausted, err := db.ReadRowStream(stream, 10)
	if err != nil || !exhausted || len(streamColumns) != 4 || len(streamRows) != 3 || streamRows[2][2] != "music" {
		t.Fatalf("unfiltered stream = columns %#v, rows %#v, exhausted %t, error %v", streamColumns, streamRows, exhausted, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, stream, err = driver.OpenTableRowStream(ctx, db.TableRowStreamRequest{Table: tables[0], Filters: []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "2"}}})
	if err != nil {
		t.Fatalf("OpenTableRowStream() with filter error = %v", err)
	}
	streamRows, exhausted, err = db.ReadRowStream(stream, 10)
	if err != nil || !exhausted || len(streamRows) != 1 || streamRows[0][2] != "games" {
		t.Fatalf("filtered stream = rows %#v, exhausted %t, error %v", streamRows, exhausted, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
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

	queryStream, err := driver.OpenQueryRowStream(ctx, db.Query{SQL: "select name from categories order by id"})
	if err != nil {
		t.Fatalf("OpenQueryRowStream() error = %v", err)
	}
	queryRows, exhausted, err := db.ReadRowStream(queryStream, 10)
	if err != nil || !exhausted || len(queryStream.Columns()) != 1 || len(queryRows) != 3 || queryRows[2][0] != "music" {
		t.Fatalf("query stream = columns %#v, rows %#v, exhausted %t, error %v", queryStream.Columns(), queryRows, exhausted, err)
	}
	if err := queryStream.Close(); err != nil {
		t.Fatalf("query stream Close() error = %v", err)
	}

	queryStream, err = driver.OpenQueryRowStream(ctx, db.Query{SQL: "update categories set note = 'streamed' where id = 1"})
	if err != nil {
		t.Fatalf("OpenQueryRowStream() for update error = %v", err)
	}
	queryRows, exhausted, err = db.ReadRowStream(queryStream, 1)
	if err != nil || !exhausted || len(queryRows) != 0 || queryStream.RowsAffected() != 1 {
		t.Fatalf("update stream = rows %#v, exhausted %t, affected %d, error %v", queryRows, exhausted, queryStream.RowsAffected(), err)
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

func TestBrowseWhereUsesParameterizedOperators(t *testing.T) {
	where, args, err := sqliteBrowseWhere([]db.RowFilter{
		{Column: "name", Operator: db.FilterEqual, Value: "Ada"},
		{Column: "note", Operator: db.FilterContains, Value: "vip%_"},
		{Column: "code", Operator: db.FilterLike, Value: "A_%"},
		{Column: "rank", Operator: db.FilterGreater, Value: "10"},
		{Column: "rank", Operator: db.FilterLess, Value: "20"},
		{Column: "rank", Operator: db.FilterGreaterOrEqual, Value: "10"},
		{Column: "rank", Operator: db.FilterLessOrEqual, Value: "20"},
		{Column: "deleted_at", Operator: db.FilterIsNull},
		{Column: "email", Operator: db.FilterIsNotNull},
	})
	if err != nil {
		t.Fatalf("sqliteBrowseWhere() error = %v", err)
	}
	wantWhere := " where \"name\" = ? and lower(cast(\"note\" as text)) like lower(?) escape '\\' and cast(\"code\" as text) like ? and \"rank\" > ? and \"rank\" < ? and \"rank\" >= ? and \"rank\" <= ? and \"deleted_at\" is null and \"email\" is not null"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 7 || args[0] != "Ada" || args[1] != "%vip\\%\\_%" || args[2] != "A_%" || args[3] != "10" || args[4] != "20" || args[5] != "10" || args[6] != "20" {
		t.Fatalf("args = %#v", args)
	}
}

func TestBrowseWhereRejectsILike(t *testing.T) {
	if _, _, err := sqliteBrowseWhere([]db.RowFilter{{Column: "name", Operator: db.FilterILike, Value: "ada%"}}); err == nil {
		t.Fatal("sqliteBrowseWhere() accepted PostgreSQL-only ILIKE")
	}
}

func TestReturnsRowsSupportsCommentsAndReturning(t *testing.T) {
	for _, query := range []string{"-- comment\nselect 1", "/* comment */ select 1", "insert into items values (1) returning id"} {
		if !returnsRows(query) {
			t.Fatalf("returnsRows(%q) = false", query)
		}
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
