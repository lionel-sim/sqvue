package db

import "context"

type DbType string

const (
	DbTypePostgres DbType = "postgres"
)

type ConnectConfig struct {
	DbType   DbType
	DSN      string // full connection string; takes precedence over the fields below
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type Driver interface {
	DbType() DbType

	// lifecycle
	Connect(ctx context.Context, cfg ConnectConfig) error
	Close() error
	Ping(ctx context.Context) error

	// metadata
	ListSchemas(ctx context.Context) ([]Schema, error)
	ListTables(ctx context.Context, schema string) ([]Table, error)
	DescribeTable(ctx context.Context, schema, table string) (TableInfo, error)

	// data
	Query(ctx context.Context, q Query) (Result, error)
	Rows(ctx context.Context, tbl Table, limit, offset int) ([]Column, [][]string, error)
	CountRows(ctx context.Context, tbl Table) (int64, error)
}

type Schema struct{ Name string }

type Table struct {
	Schema string
	Name   string
	Type   string // "table", "view", "materialized_view" etc
}

func (t Table) String() string {
	return t.Schema + "." + t.Name
}

type Column struct {
	Name      string
	DataType  string
	Nullable  bool
	Default   *string
	IsPrimary bool
}

type TableInfo struct {
	Schema  string
	Name    string
	Columns []Column
}

type Query struct {
	SQL  string
	Args []any
}

type Result struct {
	Columns      []string
	Rows         [][]any
	RowsAffected int64
	DurationMs   int64
}

func NewClient(ctx context.Context, connString string) (Driver, error) {
	return NewClientWithConfig(ctx, ConnectConfig{
		DbType: DbTypePostgres,
		DSN:    connString,
	})
}

// NewClientWithConfig connects a driver using either a DSN or individual
// connection fields. A DSN takes precedence when both are supplied.
func NewClientWithConfig(ctx context.Context, cfg ConnectConfig) (Driver, error) {
	if cfg.DbType == "" {
		cfg.DbType = DbTypePostgres
	}
	driver, err := NewDriver(cfg.DbType)
	if err != nil {
		return nil, err
	}
	if err := driver.Connect(ctx, cfg); err != nil {
		return nil, err
	}
	return driver, nil
}
