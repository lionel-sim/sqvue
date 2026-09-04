package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"sqvue/internal/db"

	_ "modernc.org/sqlite"
)

const maxQueryRows = 1_000

type Driver struct {
	db *sql.DB
}

func New() *Driver                  { return &Driver{} }
func (d *Driver) DbType() db.DbType { return db.DbTypeSQLite }

func (d *Driver) Connect(ctx context.Context, cfg db.ConnectConfig) error {
	if cfg.DSN == "" {
		return fmt.Errorf("SQLite database path is required")
	}
	client, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return err
	}
	// A single connection keeps :memory: databases consistent across requests.
	client.SetMaxOpenConns(1)
	if err := client.PingContext(ctx); err != nil {
		_ = client.Close()
		return err
	}
	previous := d.db
	d.db = client
	if previous != nil {
		return previous.Close()
	}
	return nil
}

func (d *Driver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *Driver) Ping(ctx context.Context) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	return d.db.PingContext(ctx)
}

func (d *Driver) ListSchemas(ctx context.Context) ([]db.Schema, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	rows, err := d.db.QueryContext(ctx, "pragma database_list")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []db.Schema
	for rows.Next() {
		var sequence int
		var name, path string
		if err := rows.Scan(&sequence, &name, &path); err != nil {
			return nil, err
		}
		schemas = append(schemas, db.Schema{Name: name})
	}
	return schemas, rows.Err()
}

func (d *Driver) ListTables(ctx context.Context, schema string) ([]db.Table, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	query := fmt.Sprintf(`
		select ?, name, case type when 'view' then 'view' else 'table' end
		from %s.sqlite_master
		where type in ('table', 'view') and name not like 'sqlite_%%'
		order by name`, quoteIdent(schema))
	rows, err := d.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []db.Table
	for rows.Next() {
		var table db.Table
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (d *Driver) DescribeTable(ctx context.Context, schema, table string) (db.TableInfo, error) {
	columns, err := d.columnMetadata(ctx, schema, table)
	if err != nil {
		return db.TableInfo{}, err
	}
	return db.TableInfo{Schema: schema, Name: table, Columns: columns}, nil
}

func (d *Driver) Rows(ctx context.Context, table db.Table, limit, offset int) ([]db.Column, [][]string, error) {
	columns, err := d.columnMetadata(ctx, table.Schema, table.Name)
	if err != nil {
		return nil, nil, err
	}

	query := fmt.Sprintf("select * from %s", qualifiedName(table.Schema, table.Name))
	if orderBy := rowOrder(table, columns); orderBy != "" {
		query += " order by " + orderBy
	}
	query += " limit ? offset ?"
	rows, err := d.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	var result [][]string
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(values))
		for i, value := range values {
			row[i] = formatValue(value)
		}
		result = append(result, row)
	}
	return columns, result, rows.Err()
}

func (d *Driver) RowsByColumn(ctx context.Context, table db.Table, column, value string, limit int) ([]db.Column, [][]string, error) {
	columns, err := d.columnMetadata(ctx, table.Schema, table.Name)
	if err != nil {
		return nil, nil, err
	}
	query := fmt.Sprintf("select * from %s where %s = ? limit ?", qualifiedName(table.Schema, table.Name), quoteIdent(column))
	rows, err := d.db.QueryContext(ctx, query, value, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	var result [][]string
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(values))
		for i, value := range values {
			row[i] = formatValue(value)
		}
		result = append(result, row)
	}
	return columns, result, rows.Err()
}

func (d *Driver) BrowseRows(ctx context.Context, req db.BrowseRequest) ([]db.Column, [][]string, error) {
	if len(req.Filters) == 0 {
		return d.Rows(ctx, req.Table, req.Limit, req.Offset)
	}
	if len(req.Filters) == 1 && req.Filters[0].Operator == db.FilterEqual {
		filter := req.Filters[0]
		return d.RowsByColumn(ctx, req.Table, filter.Column, filter.Value, req.Limit)
	}
	return nil, nil, fmt.Errorf("unsupported browse filter")
}

func (d *Driver) CountRows(ctx context.Context, table db.Table) (int64, error) {
	if d.db == nil {
		return 0, fmt.Errorf("not connected")
	}
	var count int64
	if err := d.db.QueryRowContext(ctx, fmt.Sprintf("select count(*) from %s", qualifiedName(table.Schema, table.Name))).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (d *Driver) Query(ctx context.Context, query db.Query) (db.Result, error) {
	if d.db == nil {
		return db.Result{}, fmt.Errorf("not connected")
	}
	started := time.Now()
	if !returnsRows(query.SQL) {
		result, err := d.db.ExecContext(ctx, query.SQL, query.Args...)
		if err != nil {
			return db.Result{}, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			affected = 0
		}
		return db.Result{RowsAffected: affected, DurationMs: time.Since(started).Milliseconds()}, nil
	}

	rows, err := d.db.QueryContext(ctx, query.SQL, query.Args...)
	if err != nil {
		return db.Result{}, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return db.Result{}, err
	}
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	var resultRows [][]any
	truncated := false
	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return db.Result{}, err
		}
		row := make([]any, len(values))
		copy(row, values)
		resultRows = append(resultRows, row)
		if len(resultRows) > maxQueryRows {
			resultRows = resultRows[:maxQueryRows]
			truncated = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return db.Result{}, err
	}
	return db.Result{
		Columns:    columns,
		Rows:       resultRows,
		DurationMs: time.Since(started).Milliseconds(),
		Truncated:  truncated,
	}, nil
}

func (d *Driver) columnMetadata(ctx context.Context, schema, table string) ([]db.Column, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	query := fmt.Sprintf("pragma %s.table_info(%s)", quoteIdent(schema), quoteIdent(table))
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []db.Column
	for rows.Next() {
		var ordinal, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&ordinal, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		column := db.Column{
			Name:      name,
			DataType:  dataType,
			Nullable:  notNull == 0 && primaryKey == 0,
			IsPrimary: primaryKey > 0,
		}
		if defaultValue.Valid {
			column.Default = &defaultValue.String
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	foreignKeys, err := d.foreignKeyMetadata(ctx, schema, table)
	if err != nil {
		return nil, err
	}
	for i := range columns {
		if foreignKey, ok := foreignKeys[columns[i].Name]; ok {
			columns[i].ForeignKey = &foreignKey
		}
	}
	return columns, nil
}

func (d *Driver) foreignKeyMetadata(ctx context.Context, schema, table string) (map[string]db.ForeignKey, error) {
	query := fmt.Sprintf("pragma %s.foreign_key_list(%s)", quoteIdent(schema), quoteIdent(table))
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	foreignKeys := make(map[string]db.ForeignKey)
	for rows.Next() {
		var id, sequence int
		var foreignTable, column, foreignColumn, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &sequence, &foreignTable, &column, &foreignColumn, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		foreignKeys[column] = db.ForeignKey{Schema: schema, Table: foreignTable, Column: foreignColumn}
	}
	return foreignKeys, rows.Err()
}

func rowOrder(table db.Table, columns []db.Column) string {
	var primary []string
	for _, column := range columns {
		if column.IsPrimary {
			primary = append(primary, quoteIdent(column.Name))
		}
	}
	if len(primary) > 0 {
		return strings.Join(primary, ", ")
	}
	if table.Type == "table" {
		return "rowid"
	}
	return ""
}

func returnsRows(query string) bool {
	statement := strings.ToLower(strings.TrimSpace(query))
	for _, prefix := range []string{"select", "with", "pragma", "explain", "values"} {
		if strings.HasPrefix(statement, prefix) {
			return true
		}
	}
	return false
}

func qualifiedName(schema, name string) string {
	return quoteIdent(schema) + "." + quoteIdent(name)
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func formatValue(value any) string {
	if value == nil {
		return "NULL"
	}
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return fmt.Sprint(value)
}

func init() {
	db.Register(db.DbTypeSQLite, func() db.Driver { return New() })
}
