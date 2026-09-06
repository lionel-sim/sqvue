package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sqvue/internal/db"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Driver struct {
	pool          *pgxpool.Pool
	backupConnect db.ConnectConfig
}

const maxQueryRows = 1_000

func New() *Driver                  { return &Driver{} }
func (d *Driver) DbType() db.DbType { return db.DbTypePostgres }

func (d *Driver) Connect(ctx context.Context, cfg db.ConnectConfig) error {
	poolConfig, err := poolConfig(cfg)
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return err
	}
	oldPool := d.pool
	d.pool, d.backupConnect = pool, cfg
	if oldPool != nil {
		oldPool.Close()
	}
	return nil
}

func poolConfig(cfg db.ConnectConfig) (*pgxpool.Config, error) {
	if cfg.DSN != "" {
		return pgxpool.ParseConfig(cfg.DSN)
	}
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   "/" + cfg.Database,
	}
	query := u.Query()
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()
	return pgxpool.ParseConfig(u.String())
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
	if d.pool == nil {
		return db.TableInfo{}, fmt.Errorf("not connected")
	}
	cols, err := d.columnMetadata(ctx, schema, table)
	if err != nil {
		return db.TableInfo{}, err
	}
	return db.TableInfo{Schema: schema, Name: table, Columns: cols}, nil
}

func (d *Driver) Query(ctx context.Context, q db.Query) (db.Result, error) {
	if d.pool == nil {
		return db.Result{}, fmt.Errorf("not connected")
	}
	start := time.Now()
	rows, err := d.pool.Query(ctx, q.SQL, q.Args...)
	if err != nil {
		return db.Result{}, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	var out [][]any
	truncated := false
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanTargets...); err != nil {
			return db.Result{}, err
		}
		row := make([]any, len(values))
		copy(row, values)
		out = append(out, row)
		if len(out) > maxQueryRows {
			out = out[:maxQueryRows]
			truncated = true
			break
		}
	}

	rowsAffected := rows.CommandTag().RowsAffected()
	durationMs := time.Since(start).Milliseconds()

	result := db.Result{
		Columns:      columns,
		Rows:         out,
		RowsAffected: rowsAffected,
		DurationMs:   durationMs,
		Truncated:    truncated,
	}
	return result, rows.Err()
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
	query := fmt.Sprintf("select * from %s as sqvue_row", ident)
	if orderBy := rowOrder(tbl, cols); orderBy != "" {
		query += " order by " + orderBy
	}
	query += " limit $1 offset $2"
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

func (d *Driver) RowsByColumn(ctx context.Context, tbl db.Table, column, value string, limit int) ([]db.Column, [][]string, error) {
	if d.pool == nil {
		return nil, nil, fmt.Errorf("not connected")
	}
	cols, err := d.columnMetadata(ctx, tbl.Schema, tbl.Name)
	if err != nil {
		return nil, nil, err
	}
	ident := pgx.Identifier{tbl.Schema, tbl.Name}.Sanitize()
	columnIdent := pgx.Identifier{column}.Sanitize()
	query := fmt.Sprintf("select * from %s as sqvue_row where sqvue_row.%s = $1 limit $2", ident, columnIdent)
	rows, err := d.pool.Query(ctx, query, value, limit)
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

func (d *Driver) BrowseRows(ctx context.Context, req db.BrowseRequest) ([]db.Column, [][]string, error) {
	if d.pool == nil {
		return nil, nil, fmt.Errorf("not connected")
	}
	columns, err := d.columnMetadata(ctx, req.Table.Schema, req.Table.Name)
	if err != nil {
		return nil, nil, err
	}
	where, args, err := postgresBrowseWhere(req.Filters)
	if err != nil {
		return nil, nil, err
	}
	ident := pgx.Identifier{req.Table.Schema, req.Table.Name}.Sanitize()
	query := fmt.Sprintf("select * from %s as sqvue_row", ident) + where
	if orderBy := rowOrder(req.Table, columns); orderBy != "" {
		query += " order by " + orderBy
	}
	query += fmt.Sprintf(" limit $%d offset $%d", len(args)+1, len(args)+2)
	args = append(args, req.Limit, req.Offset)
	rows, err := d.pool.Query(ctx, query, args...)
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

func (d *Driver) CountBrowseRows(ctx context.Context, req db.BrowseRequest) (int64, error) {
	if d.pool == nil {
		return 0, fmt.Errorf("not connected")
	}
	where, args, err := postgresBrowseWhere(req.Filters)
	if err != nil {
		return 0, err
	}
	ident := pgx.Identifier{req.Table.Schema, req.Table.Name}.Sanitize()
	var count int64
	err = d.pool.QueryRow(ctx, fmt.Sprintf("select count(*) from %s as sqvue_row", ident)+where, args...).Scan(&count)
	return count, err
}

func postgresBrowseWhere(filters []db.RowFilter) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	parts := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	for _, filter := range filters {
		column := "sqvue_row." + pgx.Identifier{filter.Column}.Sanitize()
		placeholder := fmt.Sprintf("$%d", len(args)+1)
		switch filter.Operator {
		case db.FilterEqual:
			parts = append(parts, column+" = "+placeholder)
			args = append(args, filter.Value)
		case db.FilterContains:
			parts = append(parts, "cast("+column+" as text) ilike "+placeholder+" escape E'\\\\'")
			args = append(args, "%"+escapeLikeLiteral(filter.Value)+"%")
		case db.FilterLike:
			parts = append(parts, "cast("+column+" as text) like "+placeholder)
			args = append(args, filter.Value)
		case db.FilterILike:
			parts = append(parts, "cast("+column+" as text) ilike "+placeholder)
			args = append(args, filter.Value)
		case db.FilterGreater:
			parts = append(parts, column+" > "+placeholder)
			args = append(args, filter.Value)
		case db.FilterLess:
			parts = append(parts, column+" < "+placeholder)
			args = append(args, filter.Value)
		case db.FilterGreaterOrEqual:
			parts = append(parts, column+" >= "+placeholder)
			args = append(args, filter.Value)
		case db.FilterLessOrEqual:
			parts = append(parts, column+" <= "+placeholder)
			args = append(args, filter.Value)
		case db.FilterIsNull:
			parts = append(parts, column+" is null")
		case db.FilterIsNotNull:
			parts = append(parts, column+" is not null")
		default:
			return "", nil, fmt.Errorf("unsupported browse filter %q", filter.Operator)
		}
	}
	return " where " + strings.Join(parts, " and "), args, nil
}

func escapeLikeLiteral(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}

func rowOrder(tbl db.Table, cols []db.Column) string {
	var primary []string
	for _, col := range cols {
		if col.IsPrimary {
			primary = append(primary, pgx.Identifier{col.Name}.Sanitize())
		}
	}
	if len(primary) > 0 {
		return strings.Join(primary, ", ")
	}
	if tbl.Type == "table" {
		return "ctid"
	}
	return "row_to_json(sqvue_row)::text"
}

func (d *Driver) CountRows(ctx context.Context, tbl db.Table) (int64, error) {
	if d.pool == nil {
		return 0, fmt.Errorf("not connected")
	}
	ident := pgx.Identifier{tbl.Schema, tbl.Name}.Sanitize()
	var count int64
	if err := d.pool.QueryRow(ctx, fmt.Sprintf("select count(*) from %s", ident)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	foreignKeys, err := d.foreignKeyMetadata(ctx, schema, table)
	if err != nil {
		return nil, err
	}
	for i := range cols {
		if foreignKey, ok := foreignKeys[cols[i].Name]; ok {
			cols[i].ForeignKey = &foreignKey
		}
	}
	return cols, nil
}

func (d *Driver) foreignKeyMetadata(ctx context.Context, schema, table string) (map[string]db.ForeignKey, error) {
	rows, err := d.pool.Query(ctx, `
		select kcu.column_name, ccu.table_schema, ccu.table_name, ccu.column_name
		from information_schema.table_constraints tc
		join information_schema.key_column_usage kcu
		  on kcu.constraint_catalog = tc.constraint_catalog
		 and kcu.constraint_schema = tc.constraint_schema
		 and kcu.constraint_name = tc.constraint_name
		join information_schema.referential_constraints rc
		  on rc.constraint_catalog = tc.constraint_catalog
		 and rc.constraint_schema = tc.constraint_schema
		 and rc.constraint_name = tc.constraint_name
		join information_schema.key_column_usage ccu
		  on ccu.constraint_catalog = rc.unique_constraint_catalog
		 and ccu.constraint_schema = rc.unique_constraint_schema
		 and ccu.constraint_name = rc.unique_constraint_name
		 and ccu.ordinal_position = kcu.position_in_unique_constraint
		where tc.constraint_type = 'FOREIGN KEY'
		  and tc.table_schema = $1
		  and tc.table_name = $2`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	foreignKeys := make(map[string]db.ForeignKey)
	for rows.Next() {
		var column string
		var foreignKey db.ForeignKey
		if err := rows.Scan(&column, &foreignKey.Schema, &foreignKey.Table, &foreignKey.Column); err != nil {
			return nil, err
		}
		foreignKeys[column] = foreignKey
	}
	return foreignKeys, rows.Err()
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch x := v.(type) {
	case []byte:
		return string(x)
	case pgtype.Numeric:
		return formatNumeric(x)
	case map[string]any, []any, []string:
		// jsonb / array columns decode to these shapes; render as JSON
		return marshalJSON(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func formatNumeric(n pgtype.Numeric) string {
	if !n.Valid {
		return "NULL"
	}
	if n.NaN {
		return "NaN"
	}
	if n.InfinityModifier == pgtype.Infinity {
		return "Infinity"
	}
	if n.InfinityModifier == pgtype.NegativeInfinity {
		return "-Infinity"
	}
	digits := "0"
	if n.Int != nil {
		digits = n.Int.String()
	}
	if n.Exp >= 0 {
		return digits + strings.Repeat("0", int(n.Exp))
	}
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	point := len(digits) + int(n.Exp)
	if point <= 0 {
		return sign + "0." + strings.Repeat("0", -point) + digits
	}
	return sign + digits[:point] + "." + digits[point:]
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
