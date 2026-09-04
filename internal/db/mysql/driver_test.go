package mysql

import (
	"context"
	"strings"
	"testing"

	"sqvue/internal/db"
)

func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent("order`items"); got != "`order``items`" {
		t.Fatalf("quoteIdent() = %q", got)
	}
}

func TestRowOrderUsesPrimaryKeyColumns(t *testing.T) {
	columns := []db.Column{{Name: "tenant_id", IsPrimary: true}, {Name: "id", IsPrimary: true}, {Name: "name"}}
	if got := rowOrder(columns); got != "`tenant_id`, `id`" {
		t.Fatalf("rowOrder() = %q", got)
	}
	if got := rowOrder([]db.Column{{Name: "name"}}); got != "" {
		t.Fatalf("rowOrder() without primary key = %q", got)
	}
}

func TestReturnsRows(t *testing.T) {
	for _, query := range []string{"select 1", " WITH data AS (SELECT 1) SELECT * FROM data", "show tables", "describe users", "explain select 1"} {
		if !returnsRows(query) {
			t.Fatalf("returnsRows(%q) = false", query)
		}
	}
	for _, query := range []string{"insert into users values (1)", "update users set name = 'Ada'", "delete from users"} {
		if returnsRows(query) {
			t.Fatalf("returnsRows(%q) = true", query)
		}
	}
}

func TestBrowseWhereUsesParameterizedOperators(t *testing.T) {
	where, args, err := mysqlBrowseWhere([]db.RowFilter{
		{Column: "name", Operator: db.FilterEqual, Value: "Ada"},
		{Column: "note", Operator: db.FilterContains, Value: "vip"},
		{Column: "deleted_at", Operator: db.FilterIsNull},
		{Column: "email", Operator: db.FilterIsNotNull},
	})
	if err != nil {
		t.Fatalf("mysqlBrowseWhere() error = %v", err)
	}
	wantWhere := " where `name` = ? and cast(`note` as char) like ? and `deleted_at` is null and `email` is not null"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 2 || args[0] != "Ada" || args[1] != "%vip%" {
		t.Fatalf("args = %#v", args)
	}
}

func TestDriverRejectsOperationsBeforeConnect(t *testing.T) {
	driver := New()
	if _, err := driver.Query(context.Background(), db.Query{SQL: "select 1"}); err == nil {
		t.Fatal("Query() succeeded before Connect()")
	}
	if _, err := driver.CountRows(context.Background(), db.Table{Schema: "app", Name: "users"}); err == nil {
		t.Fatal("CountRows() succeeded before Connect()")
	}
}

func TestConnectRejectsInvalidDSN(t *testing.T) {
	driver := New()
	err := driver.Connect(context.Background(), db.ConnectConfig{DSN: "not a valid dsn"})
	if err == nil || !strings.Contains(err.Error(), "parse MySQL") {
		t.Fatalf("Connect() error = %v", err)
	}
}
