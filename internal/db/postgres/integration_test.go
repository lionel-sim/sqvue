package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"sqvue/internal/db"
)

const postgresIntegrationDSNEnv = "SQVUE_TEST_POSTGRES_DSN"

func TestPostgresDriverIntegration(t *testing.T) {
	ctx, driver, schema := newPostgresIntegrationFixture(t)
	populatePostgresFixture(t, ctx, driver, schema)
	verifyPostgresSchemas(t, ctx, driver, schema)
	books := verifyPostgresTables(t, ctx, driver, schema)
	verifyPostgresDescription(t, ctx, driver, schema)
	verifyPostgresRows(t, ctx, driver, books)
	verifyPostgresBrowse(t, ctx, driver, books)
	verifyPostgresQuery(t, ctx, driver, schema)
}

func newPostgresIntegrationFixture(t *testing.T) (context.Context, *Driver, string) {
	t.Helper()
	dsn := os.Getenv(postgresIntegrationDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to run against a real PostgreSQL instance", postgresIntegrationDSNEnv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	driver := New()
	if err := driver.Connect(ctx, db.ConnectConfig{DSN: dsn}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	schema := fmt.Sprintf("sqvue_integration_%d", time.Now().UnixNano())
	mustQuery(t, ctx, driver, "create schema "+schema)
	t.Cleanup(func() { mustQuery(t, context.Background(), driver, "drop schema if exists "+schema+" cascade") })
	return ctx, driver, schema
}

func populatePostgresFixture(t *testing.T, ctx context.Context, driver *Driver, schema string) {
	t.Helper()
	mustQuery(t, ctx, driver, `create table `+schema+`.authors (
		id integer primary key,
		name text not null
	)`)
	mustQuery(t, ctx, driver, `create table `+schema+`.books (
		id integer primary key,
		author_id integer not null references `+schema+`.authors(id),
		title text not null,
		rating integer not null,
		archived_at timestamp
	)`)
	mustQuery(t, ctx, driver, `create view `+schema+`.book_titles as select id, title from `+schema+`.books`)
	mustQuery(t, ctx, driver, `insert into `+schema+`.authors (id, name) values (1, 'Ada'), (2, 'Linus')`)
	mustQuery(t, ctx, driver, `insert into `+schema+`.books (id, author_id, title, rating, archived_at) values
		(1, 1, 'Compilers', 5, null),
		(2, 1, 'Databases', 4, null),
		(3, 2, 'Networks', 2, now())`)
}

func verifyPostgresSchemas(t *testing.T, ctx context.Context, driver *Driver, schema string) {
	t.Helper()
	schemas, err := driver.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas() error = %v", err)
	}
	if !hasSchema(schemas, schema) {
		t.Fatalf("ListSchemas() did not return %q: %#v", schema, schemas)
	}
}

func verifyPostgresTables(t *testing.T, ctx context.Context, driver *Driver, schema string) db.Table {
	t.Helper()
	tables, err := driver.ListTables(ctx, schema)
	if err != nil {
		t.Fatalf("ListTables() error = %v", err)
	}
	books := findTable(t, tables, "books", "table")
	findTable(t, tables, "book_titles", "view")
	return books
}

func verifyPostgresDescription(t *testing.T, ctx context.Context, driver *Driver, schema string) {
	t.Helper()
	info, err := driver.DescribeTable(ctx, schema, "books")
	if err != nil {
		t.Fatalf("DescribeTable() error = %v", err)
	}
	if info.Schema != schema || info.Name != "books" {
		t.Fatalf("DescribeTable() = %#v", info)
	}
	authorID := findColumn(t, info.Columns, "author_id")
	if authorID.Nullable || authorID.ForeignKey == nil || *authorID.ForeignKey != (db.ForeignKey{Schema: schema, Table: "authors", Column: "id"}) {
		t.Fatalf("author_id metadata = %#v", authorID)
	}
	if !findColumn(t, info.Columns, "id").IsPrimary {
		t.Fatal("id column is not marked as primary")
	}
}

func verifyPostgresRows(t *testing.T, ctx context.Context, driver *Driver, books db.Table) {
	t.Helper()
	columns, rows, err := driver.Rows(ctx, books, 2, 0)
	if err != nil {
		t.Fatalf("Rows() error = %v", err)
	}
	if len(columns) != 5 || len(rows) != 2 || rows[0][0] != "1" || rows[1][0] != "2" {
		t.Fatalf("Rows() = columns %#v, rows %#v", columns, rows)
	}
}

func verifyPostgresBrowse(t *testing.T, ctx context.Context, driver *Driver, books db.Table) {
	t.Helper()
	req := db.BrowseRequest{Table: books, Limit: 10, Filters: []db.RowFilter{
		{Column: "rating", Operator: db.FilterGreaterOrEqual, Value: "4"},
		{Column: "rating", Operator: db.FilterLessOrEqual, Value: "5"},
		{Column: "archived_at", Operator: db.FilterIsNull},
	}}
	_, rows, err := driver.BrowseRows(ctx, req)
	if err != nil {
		t.Fatalf("BrowseRows() error = %v", err)
	}
	if len(rows) != 2 || rows[0][2] != "Compilers" || rows[1][2] != "Databases" {
		t.Fatalf("BrowseRows() = %#v", rows)
	}
	count, err := driver.CountBrowseRows(ctx, req)
	if err != nil || count != 2 {
		t.Fatalf("CountBrowseRows() = %d, %v", count, err)
	}
}

func verifyPostgresQuery(t *testing.T, ctx context.Context, driver *Driver, schema string) {
	t.Helper()
	result, err := driver.Query(ctx, db.Query{SQL: "select title from " + schema + ".books order by id"})
	if err != nil || len(result.Columns) != 1 || result.Columns[0] != "title" || len(result.Rows) != 3 {
		t.Fatalf("Query() = %#v, %v", result, err)
	}
}

func mustQuery(t *testing.T, ctx context.Context, driver *Driver, sql string) {
	t.Helper()
	if _, err := driver.Query(ctx, db.Query{SQL: sql}); err != nil {
		t.Fatalf("Query(%q) error = %v", sql, err)
	}
}

func hasSchema(schemas []db.Schema, name string) bool {
	for _, schema := range schemas {
		if schema.Name == name {
			return true
		}
	}
	return false
}

func findTable(t *testing.T, tables []db.Table, name, tableType string) db.Table {
	t.Helper()
	for _, table := range tables {
		if table.Name == name && table.Type == tableType {
			return table
		}
	}
	t.Fatalf("table %q (%s) not found in %#v", name, tableType, tables)
	return db.Table{}
}

func findColumn(t *testing.T, columns []db.Column, name string) db.Column {
	t.Helper()
	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q not found in %#v", name, columns)
	return db.Column{}
}
