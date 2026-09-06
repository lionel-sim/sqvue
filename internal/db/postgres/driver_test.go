package postgres

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"sqvue/internal/db"
)

func TestPoolConfigEscapesConnectionFields(t *testing.T) {
	cfg, err := poolConfig(db.ConnectConfig{
		Host:     "2001:db8::1",
		Port:     5432,
		User:     "user@example",
		Password: "p@ss/word?",
		Database: "my/database",
		SSLMode:  "require",
	})
	if err != nil {
		t.Fatalf("poolConfig() error = %v", err)
	}
	if got := cfg.ConnConfig.Host; got != "2001:db8::1" {
		t.Fatalf("host = %q", got)
	}
	if got := cfg.ConnConfig.User; got != "user@example" {
		t.Fatalf("user = %q", got)
	}
	if got := cfg.ConnConfig.Password; got != "p@ss/word?" {
		t.Fatalf("password = %q", got)
	}
	if got := cfg.ConnConfig.Database; got != "my/database" {
		t.Fatalf("database = %q", got)
	}
	if cfg.ConnConfig.TLSConfig == nil {
		t.Fatal("expected TLS to be configured")
	}
}

func TestPgDumpCommandUsesSafeConnectionOptions(t *testing.T) {
	command, err := pgDumpCommand(context.Background(), db.ConnectConfig{
		Host:     "db.example.com",
		Port:     5432,
		User:     "backup_user",
		Password: "not-in-command-arguments",
		Database: "warehouse",
		SSLMode:  "require",
	}, db.BackupRequest{Scope: db.BackupScopeTable, Path: "backup.sql", Table: db.Table{Schema: "reporting", Name: `order"items`}})
	if err != nil {
		t.Fatalf("pgDumpCommand() error = %v", err)
	}
	got := strings.Join(command.Args, " ")
	for _, want := range []string{"--format=plain", "--host=db.example.com", "--port=5432", "--username=backup_user", "--dbname=warehouse", `--table="reporting"."order""items"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("pg_dump arguments = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "not-in-command-arguments") {
		t.Fatalf("pg_dump arguments exposed password: %q", got)
	}
	if !containsEnv(command.Env, "PGPASSWORD=not-in-command-arguments") || !containsEnv(command.Env, "PGSSLMODE=require") {
		t.Fatalf("pg_dump environment = %#v, want password and ssl mode", command.Env)
	}
}

func TestPostgresBackupCapabilities(t *testing.T) {
	capabilities := New().BackupCapabilities()
	if !capabilities.Database || !capabilities.Table || capabilities.Schema || capabilities.FileExtension != "sql" {
		t.Fatalf("backup capabilities = %#v", capabilities)
	}
}

func TestPgDumpTablePatternQuotesIdentifiers(t *testing.T) {
	if got := pgDumpTablePattern(db.Table{Schema: `sales"2026`, Name: `order"items`}); got != `"sales""2026"."order""items"` {
		t.Fatalf("pgDumpTablePattern() = %q", got)
	}
}

func containsEnv(environment []string, value string) bool {
	for _, entry := range environment {
		if entry == value {
			return true
		}
	}
	return false
}

func TestRowOrder(t *testing.T) {
	if got := rowOrder(db.Table{Type: "table"}, []db.Column{{Name: "payload"}}); got != "ctid" {
		t.Fatalf("table fallback = %q", got)
	}
	if got := rowOrder(db.Table{Type: "view"}, []db.Column{{Name: "payload"}}); got != "row_to_json(sqvue_row)::text" {
		t.Fatalf("view fallback = %q", got)
	}
	if got := rowOrder(db.Table{Type: "table"}, []db.Column{{Name: "tenant", IsPrimary: true}, {Name: "id", IsPrimary: true}}); got != `"tenant", "id"` {
		t.Fatalf("primary order = %q", got)
	}
}

func TestSortedQueryQuotesPostgresIdentifiers(t *testing.T) {
	query, err := New().SortedQuery(db.Query{SQL: "select * from entries;"}, db.SortSpec{Column: `name" desc`, Descending: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := `select * from (select * from entries) as sqvue_query order by "name"" desc" desc`; query.SQL != want {
		t.Fatalf("SortedQuery() = %q, want %q", query.SQL, want)
	}
}

func TestUpdateCellStatementUsesParametersAndPrimaryKey(t *testing.T) {
	query, args, err := postgresUpdateCellStatement(db.CellUpdateRequest{
		Table:      db.Table{Schema: "public", Name: "order items"},
		Column:     "note",
		Value:      "updated",
		PrimaryKey: []db.PrimaryKeyValue{{Column: "tenant_id", Value: "north"}, {Column: "id", Value: "7"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `update "public"."order items" set "note" = $1 where "tenant_id" = $2 and "id" = $3`; query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
	if len(args) != 3 || args[0] != "updated" || args[1] != "north" || args[2] != "7" {
		t.Fatalf("args = %#v", args)
	}
}

func TestInsertRowStatementUsesParametersAndTypedValues(t *testing.T) {
	request := db.RowInsertRequest{Table: db.Table{Schema: "public", Name: "order items"}, Values: []db.RowInsertValue{
		{Column: "note", Kind: db.RowInsertLiteral, Value: "created"},
		{Column: "deleted_at", Kind: db.RowInsertNull},
		{Column: "created_at", Kind: db.RowInsertCurrentTimestamp},
	}}
	query, args, err := postgresInsertRowStatement(request)
	if err != nil {
		t.Fatal(err)
	}
	if want := `insert into "public"."order items" ("note", "deleted_at", "created_at") values ($1, null, current_timestamp)`; query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
	if len(args) != 1 || args[0] != "created" {
		t.Fatalf("args = %#v", args)
	}
	query, args, err = postgresInsertRowStatement(db.RowInsertRequest{Table: request.Table})
	if err != nil || query != `insert into "public"."order items" default values` || len(args) != 0 {
		t.Fatalf("default insert = %q, %#v, %v", query, args, err)
	}
}

func TestDeleteRowStatementUsesParametersAndPrimaryKey(t *testing.T) {
	query, args, err := postgresDeleteRowStatement(db.RowDeleteRequest{
		Table:      db.Table{Schema: "public", Name: "order items"},
		PrimaryKey: []db.PrimaryKeyValue{{Column: "tenant_id", Value: "north"}, {Column: "id", Value: "7"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := `delete from "public"."order items" where "tenant_id" = $1 and "id" = $2`; query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
	if len(args) != 2 || args[0] != "north" || args[1] != "7" {
		t.Fatalf("args = %#v", args)
	}
}

func TestBrowseWhereUsesParameterizedOperators(t *testing.T) {
	where, args, err := postgresBrowseWhere([]db.RowFilter{
		{Column: "name", Operator: db.FilterEqual, Value: "Ada"},
		{Column: "note", Operator: db.FilterContains, Value: "vip%_"},
		{Column: "code", Operator: db.FilterLike, Value: "A_%"},
		{Column: "label", Operator: db.FilterILike, Value: "demo%"},
		{Column: "rank", Operator: db.FilterGreater, Value: "10"},
		{Column: "rank", Operator: db.FilterLess, Value: "20"},
		{Column: "rank", Operator: db.FilterGreaterOrEqual, Value: "10"},
		{Column: "rank", Operator: db.FilterLessOrEqual, Value: "20"},
		{Column: "deleted_at", Operator: db.FilterIsNull},
		{Column: "email", Operator: db.FilterIsNotNull},
	})
	if err != nil {
		t.Fatalf("postgresBrowseWhere() error = %v", err)
	}
	wantWhere := " where sqvue_row.\"name\" = $1 and cast(sqvue_row.\"note\" as text) ilike $2 escape E'\\\\' and cast(sqvue_row.\"code\" as text) like $3 and cast(sqvue_row.\"label\" as text) ilike $4 and sqvue_row.\"rank\" > $5 and sqvue_row.\"rank\" < $6 and sqvue_row.\"rank\" >= $7 and sqvue_row.\"rank\" <= $8 and sqvue_row.\"deleted_at\" is null and sqvue_row.\"email\" is not null"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 8 || args[0] != "Ada" || args[1] != "%vip\\%\\_%" || args[2] != "A_%" || args[3] != "demo%" || args[4] != "10" || args[5] != "20" || args[6] != "10" || args[7] != "20" {
		t.Fatalf("args = %#v", args)
	}
}

func TestFormatNumericPreservesPrecision(t *testing.T) {
	n := pgtype.Numeric{Int: new(big.Int), Exp: -3, Valid: true}
	n.Int.SetString("12345678901234567890123", 10)
	if got := formatNumeric(n); got != "12345678901234567890.123" {
		t.Fatalf("formatNumeric() = %q", got)
	}
}
