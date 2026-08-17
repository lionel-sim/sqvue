package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sqvue/internal/db"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Driver struct {
	pool *pgxpool.Pool
}

func New() *Driver                  { return &Driver{} }
func (d *Driver) DbType() db.DbType { return db.DbTypePostgres }

func (d *Driver) Connect(ctx context.Context, cfg db.ConnectConfig) error {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
		)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	d.pool = pool
	return nil
}

func (d *Driver) Close() error {
	if d.pool != nil {
		d.pool.Close()
	}
	return nil
}

func (d *Driver) Ping(ctx context.Context) error {
	if d.pool == nil {
		return fmt.Errorf("not connected")
	}
	return d.pool.Ping(ctx)
}

func (d *Driver) ListSchemas(ctx context.Context) ([]db.Schema, error) {
	if d.pool == nil {
		return nil, fmt.Errorf("not connected")
	}
	rows, err := d.pool.Query(ctx, `
		select schema_name
		from information_schema.schemata
		where schema_name not in ('pg_catalog', 'information_schema')
		order by schema_name;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []db.Schema
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		schemas = append(schemas, db.Schema{Name: name})
	}
	return schemas, rows.Err()
}

func (d *Driver) ListTables(ctx context.Context, schema string) ([]db.Table, error) {
	if d.pool == nil {
		return nil, fmt.Errorf("not connected")
	}
	rows, err := d.pool.Query(ctx, `
		select table_schema, table_name, table_type
		from information_schema.tables
		where table_schema = $1
		  and table_type in ('BASE TABLE', 'VIEW')
		order by table_name;
	`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []db.Table
	for rows.Next() {
		var t db.Table
		var tableType string
		if err := rows.Scan(&t.Schema, &t.Name, &tableType); err != nil {
			return nil, err
		}
		switch tableType {
		case "BASE TABLE":
			t.Type = "table"
		case "VIEW":
			t.Type = "view"
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (d *Driver) DescribeTable(ctx context.Context, schema, table string) (db.TableInfo, error) {
	// TODO
	return db.TableInfo{}, nil
}

func (d *Driver) Query(ctx context.Context, q db.Query) (db.Result, error) {
	// Implementation here...
	return db.Result{}, nil
}

func (d *Driver) Rows(ctx context.Context, tbl db.Table, limit, offset int) ([]db.Column, [][]string, error) {
	if d.pool == nil {
		return nil, nil, fmt.Errorf("not connected")
	}

	cols, err := d.columnMetadata(ctx, tbl.Schema, tbl.Name)
	if err != nil {
		return nil, nil, err
	}

	ident := pgx.Identifier{tbl.Schema, tbl.Name}.Sanitize()
	query := fmt.Sprintf("select * from %s order by 1 limit $1 offset $2", ident)
	rows, err := d.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	values := make([]any, len(cols))
	scanTargets := make([]any, len(cols))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	var out [][]string
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = formatValue(v)
		}
		out = append(out, row)
	}
	return cols, out, rows.Err()
}

func (d *Driver) columnMetadata(ctx context.Context, schema, table string) ([]db.Column, error) {
	rows, err := d.pool.Query(ctx, `
		select c.column_name, c.data_type, c.is_nullable, c.column_default,
		       (kcu.column_name is not null) as is_primary
		from information_schema.columns c
		left join information_schema.table_constraints tc
		  on tc.table_schema = c.table_schema
		 and tc.table_name = c.table_name
		 and tc.constraint_type = 'PRIMARY KEY'
		left join information_schema.key_column_usage kcu
		  on kcu.constraint_name = tc.constraint_name
		 and kcu.table_schema = c.table_schema
		 and kcu.table_name = c.table_name
		 and kcu.column_name = c.column_name
		where c.table_schema = $1
		  and c.table_name = $2
		order by c.ordinal_position;
	`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []db.Column
	for rows.Next() {
		var (
			c         db.Column
			nullable  string
			def       *string
			isPrimary bool
		)
		if err := rows.Scan(&c.Name, &c.DataType, &nullable, &def, &isPrimary); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		c.Default = def
		c.IsPrimary = isPrimary
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch x := v.(type) {
	case []byte:
		return string(x)
	case pgtype.Numeric:
		f8, err := x.Float64Value()
		if err != nil || !f8.Valid {
			return "NULL"
		}
		return strconv.FormatFloat(f8.Float64, 'f', -1, 64)
	case map[string]any, []any, []string:
		// jsonb / array columns decode to these shapes; render as JSON
		return marshalJSON(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func init() {
	db.Register(db.DbTypePostgres, func() db.Driver { return New() })
}
