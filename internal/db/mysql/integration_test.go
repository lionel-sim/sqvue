package mysql

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"sqvue/internal/db"
)

const mysqlIntegrationDSNEnv = "SQVUE_TEST_MYSQL_DSN"

func TestMySQLDriverIntegration(t *testing.T) {
	dsn := os.Getenv(mysqlIntegrationDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to run against a real MySQL instance", mysqlIntegrationDSNEnv)
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil || config.DBName == "" {
		t.Fatalf("%s must contain a database name: %v", mysqlIntegrationDSNEnv, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	driver := New()
	if err := driver.Connect(ctx, db.ConnectConfig{DSN: dsn}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = driver.Close() })

	prefix := fmt.Sprintf("sqvue_integration_%d", time.Now().UnixNano())
	authors := prefix + "_authors"
	books := prefix + "_books"
	bookTitles := prefix + "_book_titles"
	t.Cleanup(func() {
		mustMySQLQuery(t, context.Background(), driver, "drop view if exists "+bookTitles)
		mustMySQLQuery(t, context.Background(), driver, "drop table if exists "+books)
		mustMySQLQuery(t, context.Background(), driver, "drop table if exists "+authors)
	})
	mustMySQLQuery(t, ctx, driver, `create table `+authors+` (
		id integer primary key,
		name text not null
	)`)
	mustMySQLQuery(t, ctx, driver, `create table `+books+` (
		id integer primary key,
		author_id integer not null,
		title text not null,
		rating integer not null,
		archived_at datetime null,
		foreign key (author_id) references `+authors+`(id)
	)`)
	mustMySQLQuery(t, ctx, driver, `create view `+bookTitles+` as select id, title from `+books)
	mustMySQLQuery(t, ctx, driver, `insert into `+authors+` (id, name) values (1, 'Ada'), (2, 'Linus')`)
	mustMySQLQuery(t, ctx, driver, `insert into `+books+` (id, author_id, title, rating, archived_at) values
		(1, 1, 'Compilers', 5, null),
		(2, 1, 'Databases', 4, null),
		(3, 2, 'Networks', 2, now())`)

	schemas, err := driver.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas() error = %v", err)
	}
	if !hasMySQLSchema(schemas, config.DBName) {
		t.Fatalf("ListSchemas() did not return %q: %#v", config.DBName, schemas)
	}

	tables, err := driver.ListTables(ctx, config.DBName)
	if err != nil {
		t.Fatalf("ListTables() error = %v", err)
	}
	bookTable := findMySQLTable(t, tables, books, "table")
	findMySQLTable(t, tables, bookTitles, "view")

	info, err := driver.DescribeTable(ctx, config.DBName, books)
	if err != nil {
		t.Fatalf("DescribeTable() error = %v", err)
	}
	authorID := findMySQLColumn(t, info.Columns, "author_id")
	if authorID.Nullable || authorID.ForeignKey == nil || *authorID.ForeignKey != (db.ForeignKey{Schema: config.DBName, Table: authors, Column: "id"}) {
		t.Fatalf("author_id metadata = %#v", authorID)
	}
	if !findMySQLColumn(t, info.Columns, "id").IsPrimary {
		t.Fatal("id column is not marked as primary")
	}

	columns, rows, err := driver.Rows(ctx, bookTable, 2, 0)
	if err != nil {
		t.Fatalf("Rows() error = %v", err)
	}
	if len(columns) != 5 || len(rows) != 2 || rows[0][0] != "1" || rows[1][0] != "2" {
		t.Fatalf("Rows() = columns %#v, rows %#v", columns, rows)
	}

	req := db.BrowseRequest{Table: bookTable, Limit: 10, Filters: []db.RowFilter{
		{Column: "rating", Operator: db.FilterGreaterOrEqual, Value: "4"},
		{Column: "rating", Operator: db.FilterLessOrEqual, Value: "5"},
		{Column: "archived_at", Operator: db.FilterIsNull},
	}}
	_, rows, err = driver.BrowseRows(ctx, req)
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

	result, err := driver.Query(ctx, db.Query{SQL: "select title from " + books + " order by id"})
	if err != nil || len(result.Columns) != 1 || result.Columns[0] != "title" || len(result.Rows) != 3 {
		t.Fatalf("Query() = %#v, %v", result, err)
	}
}

func mustMySQLQuery(t *testing.T, ctx context.Context, driver *Driver, sql string) {
	t.Helper()
	if _, err := driver.Query(ctx, db.Query{SQL: sql}); err != nil {
		t.Fatalf("Query(%q) error = %v", sql, err)
	}
}

func hasMySQLSchema(schemas []db.Schema, name string) bool {
	for _, schema := range schemas {
		if schema.Name == name {
			return true
		}
	}
	return false
}

func findMySQLTable(t *testing.T, tables []db.Table, name, tableType string) db.Table {
	t.Helper()
	for _, table := range tables {
		if table.Name == name && table.Type == tableType {
			return table
		}
	}
	t.Fatalf("table %q (%s) not found in %#v", name, tableType, tables)
	return db.Table{}
}

func findMySQLColumn(t *testing.T, columns []db.Column, name string) db.Column {
	t.Helper()
	for _, column := range columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q not found in %#v", name, columns)
	return db.Column{}
}
