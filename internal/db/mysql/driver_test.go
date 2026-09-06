package mysql

import (
	"context"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"

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
		t.Fatalf("mysqlBrowseWhere() error = %v", err)
	}
	wantWhere := " where `name` = ? and lower(cast(`note` as char)) like lower(?) escape '\\\\' and cast(`code` as char) like ? and `rank` > ? and `rank` < ? and `rank` >= ? and `rank` <= ? and `deleted_at` is null and `email` is not null"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 7 || args[0] != "Ada" || args[1] != "%vip\\%\\_%" || args[2] != "A_%" || args[3] != "10" || args[4] != "20" || args[5] != "10" || args[6] != "20" {
		t.Fatalf("args = %#v", args)
	}
}

func TestBrowseWhereRejectsILike(t *testing.T) {
	if _, _, err := mysqlBrowseWhere([]db.RowFilter{{Column: "name", Operator: db.FilterILike, Value: "ada%"}}); err == nil {
		t.Fatal("mysqlBrowseWhere() accepted PostgreSQL-only ILIKE")
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

func TestMySQLDumpArgsUseSafeConnectionOptions(t *testing.T) {
	config := &mysqldriver.Config{User: "backup_user", Passwd: "not-in-command-arguments", Net: "tcp", Addr: "db.example.com:3306", DBName: "warehouse"}
	args, err := mySQLDumpArgs(config, db.BackupRequest{Scope: db.BackupScopeDatabase, Path: "backup.sql"}, "/tmp/credentials.cnf")
	if err != nil {
		t.Fatalf("mySQLDumpArgs() error = %v", err)
	}
	got := strings.Join(args, " ")
	for _, want := range []string{"--defaults-extra-file=/tmp/credentials.cnf", "--single-transaction", "--routines", "--events", "--triggers", "--user=backup_user", "--host=db.example.com", "--port=3306", "--databases warehouse"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mysqldump arguments = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, config.Passwd) {
		t.Fatalf("mysqldump arguments exposed password: %q", got)
	}
}

func TestMySQLDumpArgsUseRequestedTableSchema(t *testing.T) {
	config := &mysqldriver.Config{User: "backup_user", Net: "tcp", Addr: "127.0.0.1:3306", DBName: "default_schema"}
	args, err := mySQLDumpArgs(config, db.BackupRequest{Scope: db.BackupScopeTable, Path: "backup.sql", Table: db.Table{Schema: "reporting", Name: "order items"}}, "/tmp/credentials.cnf")
	if err != nil {
		t.Fatalf("mySQLDumpArgs() error = %v", err)
	}
	if got := strings.Join(args, " "); !strings.HasSuffix(got, "reporting order items") {
		t.Fatalf("mysqldump table arguments = %q", got)
	}
}

func TestMySQLBackupCapabilities(t *testing.T) {
	capabilities := New().BackupCapabilities()
	if !capabilities.Database || !capabilities.Table || capabilities.Schema || capabilities.FileExtension != "sql" {
		t.Fatalf("backup capabilities = %#v", capabilities)
	}
}
