package postgres

import (
	"sqvue/internal/db"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Driver struct {
	pool *pgxpool.Pool
}

func New() *Driver                  { return &Driver{} }
func (d *Driver) DbType() db.DbType { return db.DbTypePostgres }

func (d *Driver) Connect(ctx context.Context, cfg db.ConnectConfig) error {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
	)
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
	// TODO
	return nil, nil
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
	// TODO
	return nil, nil, nil
}

func init() {
	db.Register(db.DbTypePostgres, func() db.Driver { return New() })
}
