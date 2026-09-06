package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"sqvue/internal/db"
)

func TestDriverBrowsesAndQueriesSQLite(t *testing.T) {
	ctx, driver, table := setupSQLiteBrowseTest(t)
	t.Run("metadata", func(t *testing.T) { assertSQLiteMetadata(t, ctx, driver) })
	t.Run("browse", func(t *testing.T) { assertSQLiteBrowse(t, ctx, driver, table) })
	t.Run("streams", func(t *testing.T) { assertSQLiteStreams(t, ctx, driver, table) })
	t.Run("query and update", func(t *testing.T) { assertSQLiteQueryAndUpdate(t, ctx, driver, table) })
}

func setupSQLiteBrowseTest(t *testing.T) (context.Context, *Driver, db.Table) {
	t.Helper()
	ctx := context.Background()
	driver := New()
	if err := driver.Connect(ctx, db.ConnectConfig{DSN: filepath.Join(t.TempDir(), "sqvue.db")}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	for _, sql := range []string{`create table categories (id integer primary key, parent_id integer references categories(id), name text not null, note text default 'none')`, `insert into categories (name) values ('books'), ('games'), ('music')`, `create view category_names as select name from categories`} {
		if _, err := driver.Query(ctx, db.Query{SQL: sql}); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, driver, db.Table{Schema: "main", Name: "categories", Type: "table"}
}
func assertSQLiteMetadata(t *testing.T, ctx context.Context, d *Driver) {
	t.Helper()
	schemas, err := d.ListSchemas(ctx)
	if err != nil || len(schemas) == 0 || schemas[0].Name != "main" {
		t.Fatalf("schemas = %#v, %v", schemas, err)
	}
	info, err := d.DescribeTable(ctx, "main", "categories")
	if err != nil || len(info.Columns) != 4 || !info.Columns[0].IsPrimary || info.Columns[1].ForeignKey == nil {
		t.Fatalf("columns = %#v, %v", info.Columns, err)
	}
	tables, err := d.ListTables(ctx, "main")
	if err != nil || len(tables) != 2 || tables[1].Name != "category_names" || tables[1].Type != "view" {
		t.Fatalf("tables = %#v, %v", tables, err)
	}
}
func assertSQLiteBrowse(t *testing.T, ctx context.Context, d *Driver, table db.Table) {
	t.Helper()
	cols, rows, err := d.Rows(ctx, table, 2, 0)
	if err != nil || len(cols) != 4 || len(rows) != 2 || rows[0][2] != "books" {
		t.Fatalf("Rows() = %#v, %v", rows, err)
	}
	_, rows, err = d.BrowseRows(ctx, db.BrowseRequest{Table: table, Limit: 2, Sort: db.SortSpec{Column: "name", Descending: true}})
	if err != nil || len(rows) != 2 || rows[0][2] != "music" {
		t.Fatalf("BrowseRows() = %#v, %v", rows, err)
	}
	_, rows, err = d.BrowseRows(ctx, db.BrowseRequest{Table: table, Limit: 2, Filters: []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "2"}}})
	if err != nil || len(rows) != 1 || rows[0][2] != "games" {
		t.Fatalf("filtered BrowseRows() = %#v, %v", rows, err)
	}
}
func assertSQLiteStreams(t *testing.T, ctx context.Context, d *Driver, table db.Table) {
	t.Helper()
	cols, stream, err := d.OpenTableRowStream(ctx, db.TableRowStreamRequest{Table: table})
	if err != nil {
		t.Fatal(err)
	}
	rows, exhausted, err := db.ReadRowStream(stream, 10)
	if err != nil || !exhausted || len(cols) != 4 || len(rows) != 3 {
		t.Fatalf("stream = %#v, %v", rows, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	_, stream, err = d.OpenTableRowStream(ctx, db.TableRowStreamRequest{Table: table, Filters: []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "2"}}})
	if err != nil {
		t.Fatal(err)
	}
	rows, exhausted, err = db.ReadRowStream(stream, 10)
	if err != nil || !exhausted || len(rows) != 1 || rows[0][2] != "games" {
		t.Fatalf("filtered stream = %#v, %v", rows, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}
func assertSQLiteQueryAndUpdate(t *testing.T, ctx context.Context, d *Driver, table db.Table) {
	t.Helper()
	result, err := d.Query(ctx, db.Query{SQL: "select name from categories order by id"})
	if err != nil || len(result.Rows) != 3 {
		t.Fatalf("query = %#v, %v", result, err)
	}
	stream, err := d.OpenQueryRowStream(ctx, db.Query{SQL: "select name from categories order by id"})
	if err != nil {
		t.Fatal(err)
	}
	rows, exhausted, err := db.ReadRowStream(stream, 10)
	if err != nil || !exhausted || len(rows) != 3 {
		t.Fatalf("query stream = %#v, %v", rows, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateCell(ctx, db.CellUpdateRequest{Table: table, Column: "note", Value: "edited", PrimaryKey: []db.PrimaryKeyValue{{Column: "id", Value: "1"}}}); err != nil {
		t.Fatal(err)
	}
	result, err = d.Query(ctx, db.Query{SQL: "select note from categories where id = 1"})
	if err != nil || result.Rows[0][0] != "edited" {
		t.Fatalf("update = %#v, %v", result, err)
	}
}

func TestSortedQueryQuotesSQLiteIdentifiers(t *testing.T) {
	query, err := New().SortedQuery(db.Query{SQL: "select * from entries;"}, db.SortSpec{Column: `name" desc`, Descending: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := `select * from (select * from entries) as sqvue_query order by "name"" desc" desc`; query.SQL != want {
		t.Fatalf("SortedQuery() = %q, want %q", query.SQL, want)
	}
}

func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent(`a"b`); got != `"a""b"` {
		t.Fatalf("quoteIdent() = %q", got)
	}
}

func TestUpdateCellSupportsCompositeKeysNullsTypesAndCancellation(t *testing.T) {
	ctx := context.Background()
	driver := New()
	if err := driver.Connect(ctx, db.ConnectConfig{DSN: filepath.Join(t.TempDir(), "updates.db")}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	for _, statement := range []string{
		"create table items (tenant integer not null, id integer not null, label text, amount integer not null, primary key (tenant, id))",
		"insert into items (tenant, id, label, amount) values (7, 9, 'before', 1)",
	} {
		if _, err := driver.Query(ctx, db.Query{SQL: statement}); err != nil {
			t.Fatalf("Query(%q) error = %v", statement, err)
		}
	}
	request := db.CellUpdateRequest{
		Table:      db.Table{Schema: "main", Name: "items"},
		Column:     "label",
		Value:      nil,
		PrimaryKey: []db.PrimaryKeyValue{{Column: "tenant", Value: "7"}, {Column: "id", Value: "9"}},
	}
	if err := driver.UpdateCell(ctx, request); err != nil {
		t.Fatalf("UpdateCell(NULL) error = %v", err)
	}
	request.Column, request.Value = "amount", "42"
	if err := driver.UpdateCell(ctx, request); err != nil {
		t.Fatalf("UpdateCell(integer) error = %v", err)
	}
	result, err := driver.Query(ctx, db.Query{SQL: "select typeof(label), typeof(amount), amount from items where tenant = 7 and id = 9"})
	if err != nil || len(result.Rows) != 1 || result.Rows[0][0] != "null" || result.Rows[0][1] != "integer" || result.Rows[0][2] != int64(42) {
		t.Fatalf("updated values = %#v, error = %v", result.Rows, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := driver.UpdateCell(cancelled, request); err == nil {
		t.Fatal("UpdateCell() succeeded with a cancelled context")
	}
}

func TestUpdateCellStatementUsesParametersAndPrimaryKey(t *testing.T) {
	query, args, err := sqliteUpdateCellStatement(db.CellUpdateRequest{
		Table:      db.Table{Schema: "main", Name: "order items"},
		Column:     "note",
		Value:      "updated",
		PrimaryKey: []db.PrimaryKeyValue{{Column: "tenant_id", Value: "north"}, {Column: "id", Value: "7"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `update "main"."order items" set "note" = ? where "tenant_id" = ? and "id" = ?`; query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
	if len(args) != 3 || args[0] != "updated" || args[1] != "north" || args[2] != "7" {
		t.Fatalf("args = %#v", args)
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
