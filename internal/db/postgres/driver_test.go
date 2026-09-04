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
	if got := rowOrder(db.Table{Type: "view"}, []db.Column{{Name: "payload"}}); got != "" {
		t.Fatalf("view fallback = %q", got)
	}
	if got := rowOrder(db.Table{Type: "table"}, []db.Column{{Name: "tenant", IsPrimary: true}, {Name: "id", IsPrimary: true}}); got != `"tenant", "id"` {
		t.Fatalf("primary order = %q", got)
	}
}

func TestFormatNumericPreservesPrecision(t *testing.T) {
	n := pgtype.Numeric{Int: new(big.Int), Exp: -3, Valid: true}
	n.Int.SetString("12345678901234567890123", 10)
	if got := formatNumeric(n); got != "12345678901234567890.123" {
		t.Fatalf("formatNumeric() = %q", got)
	}
}
