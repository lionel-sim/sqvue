package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"sqvue/internal/db"
)

func TestSQLiteDatabaseBackupCopiesDatabase(t *testing.T) {
	ctx := context.Background()
	source := New()
	if err := source.Connect(ctx, db.ConnectConfig{DSN: filepath.Join(t.TempDir(), "source.db")}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	for _, statement := range []string{
		`create table accounts (id integer primary key, name text not null)`,
		`insert into accounts values (1, 'Ada')`,
	} {
		if _, err := source.Query(ctx, db.Query{SQL: statement}); err != nil {
			t.Fatalf("source query %q: %v", statement, err)
		}
	}
	path := filepath.Join(t.TempDir(), "database.db")
	if err := source.Backup(ctx, db.BackupRequest{Scope: db.BackupScopeDatabase, Path: path}); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	target := New()
	if err := target.Connect(ctx, db.ConnectConfig{DSN: path}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = target.Close() })
	result, err := target.Query(ctx, db.Query{SQL: "select name from accounts"})
	if err != nil || len(result.Rows) != 1 || result.Rows[0][0] != "Ada" {
		t.Fatalf("backup rows = %#v, error = %v", result.Rows, err)
	}
}

func TestSQLiteTableBackupPreservesTableObjects(t *testing.T) {
	ctx := context.Background()
	source := New()
	if err := source.Connect(ctx, db.ConnectConfig{DSN: filepath.Join(t.TempDir(), "source.db")}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	for _, statement := range []string{
		`create table orders (id integer primary key, code text not null unique, amount integer not null check (amount > 0))`,
		`create index orders_by_amount on orders(amount)`,
		`create trigger orders_before_insert before insert on orders when new.code = '' begin select raise(abort, 'code required'); end`,
		`insert into orders values (1, 'A-1', 10)`,
	} {
		if _, err := source.Query(ctx, db.Query{SQL: statement}); err != nil {
			t.Fatalf("source query %q: %v", statement, err)
		}
	}
	path := filepath.Join(t.TempDir(), "orders.db")
	if err := source.Backup(ctx, db.BackupRequest{Scope: db.BackupScopeTable, Path: path, Table: db.Table{Schema: "main", Name: "orders"}}); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	target := New()
	if err := target.Connect(ctx, db.ConnectConfig{DSN: path}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = target.Close() })
	result, err := target.Query(ctx, db.Query{SQL: "select code, amount from orders"})
	if err != nil || len(result.Rows) != 1 || result.Rows[0][0] != "A-1" || fmt.Sprint(result.Rows[0][1]) != "10" {
		t.Fatalf("backup rows = %#v, error = %v", result.Rows, err)
	}
	if _, err := target.Query(ctx, db.Query{SQL: "insert into orders values (2, 'B-2', 0)"}); err == nil {
		t.Fatal("backup table did not preserve check constraint")
	}
	for _, object := range []string{"orders_by_amount", "orders_before_insert"} {
		result, err := target.Query(ctx, db.Query{SQL: "select name from sqlite_master where name = '" + object + "'"})
		if err != nil || len(result.Rows) != 1 {
			t.Fatalf("backup missing %s: rows %#v, error = %v", object, result.Rows, err)
		}
	}
}

func TestSQLiteBackupCapabilities(t *testing.T) {
	capabilities := New().BackupCapabilities()
	if !capabilities.Database || !capabilities.Table || capabilities.Schema || capabilities.FileExtension != "db" {
		t.Fatalf("backup capabilities = %#v", capabilities)
	}
}

func TestQualifySQLiteDDL(t *testing.T) {
	statement, err := qualifySQLiteDDL(`CREATE UNIQUE INDEX "orders by code" ON orders(code)`, createIndexPrefix, "backup")
	if err != nil {
		t.Fatal(err)
	}
	if want := `CREATE UNIQUE INDEX "backup"."orders by code" ON orders(code)`; statement != want {
		t.Fatalf("qualifySQLiteDDL() = %q, want %q", statement, want)
	}
}
