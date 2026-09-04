package postgres

import (
	"math/big"
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

func TestBrowseWhereUsesParameterizedOperators(t *testing.T) {
	where, args, err := postgresBrowseWhere([]db.RowFilter{
		{Column: "name", Operator: db.FilterEqual, Value: "Ada"},
		{Column: "note", Operator: db.FilterContains, Value: "vip"},
		{Column: "code", Operator: db.FilterLike, Value: "A_%"},
		{Column: "rank", Operator: db.FilterGreater, Value: "10"},
		{Column: "rank", Operator: db.FilterLess, Value: "20"},
		{Column: "deleted_at", Operator: db.FilterIsNull},
		{Column: "email", Operator: db.FilterIsNotNull},
	})
	if err != nil {
		t.Fatalf("postgresBrowseWhere() error = %v", err)
	}
	wantWhere := " where sqvue_row.\"name\" = $1 and cast(sqvue_row.\"note\" as text) ilike $2 and cast(sqvue_row.\"code\" as text) like $3 and sqvue_row.\"rank\" > $4 and sqvue_row.\"rank\" < $5 and sqvue_row.\"deleted_at\" is null and sqvue_row.\"email\" is not null"
	if where != wantWhere {
		t.Fatalf("where = %q, want %q", where, wantWhere)
	}
	if len(args) != 5 || args[0] != "Ada" || args[1] != "%vip%" || args[2] != "A_%" || args[3] != "10" || args[4] != "20" {
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
