package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"sqvue/internal/db"
)

const maxQueryRows = 1_000

type Driver struct {
	db           *sql.DB
	backupConfig *mysqldriver.Config
}

var _ db.TableRowStreamer = (*Driver)(nil)
var _ db.QueryRowStreamer = (*Driver)(nil)
var _ db.QuerySorter = (*Driver)(nil)
var _ db.CellUpdater = (*Driver)(nil)

func New() *Driver                  { return &Driver{} }
func (d *Driver) DbType() db.DbType { return db.DbTypeMySQL }

func (d *Driver) Connect(ctx context.Context, cfg db.ConnectConfig) error {
	if cfg.DSN == "" {
		return fmt.Errorf("MySQL connection string is required")
	}
	parsed, err := mysqldriver.ParseDSN(cfg.DSN)
	if err != nil {
		return fmt.Errorf("parse MySQL connection string: %w", err)
	}
	client, err := sql.Open("mysql", parsed.FormatDSN())
	if err != nil {
		return err
	}
	if err := client.PingContext(ctx); err != nil {
		_ = client.Close()
		return err
	}
	previous := d.db
	d.db, d.backupConfig = client, parsed
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
	rows, err := d.db.QueryContext(ctx, `
		select schema_name
		from information_schema.schemata
		where schema_name not in ('information_schema', 'mysql', 'performance_schema', 'sys')
		order by schema_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []db.Schema
	for rows.Next() {
		var schema db.Schema
		if err := rows.Scan(&schema.Name); err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	return schemas, rows.Err()
}

func (d *Driver) ListTables(ctx context.Context, schema string) ([]db.Table, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	rows, err := d.db.QueryContext(ctx, `
		select table_schema, table_name,
		       case table_type when 'VIEW' then 'view' else 'table' end
		from information_schema.tables
		where table_schema = ? and table_type in ('BASE TABLE', 'VIEW')
		order by table_name`, schema)
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
	if orderBy := rowOrder(columns); orderBy != "" {
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
	columns, err := d.columnMetadata(ctx, req.Table.Schema, req.Table.Name)
	if err != nil {
		return nil, nil, err
	}
	where, args, err := mysqlBrowseWhere(req.Filters)
	if err != nil {
		return nil, nil, err
	}
	query := fmt.Sprintf("select * from %s", qualifiedName(req.Table.Schema, req.Table.Name)) + where
	if orderBy := sortOrder(columns, req.Sort); orderBy != "" {
		query += " order by " + orderBy
	}
	query += " limit ? offset ?"
	args = append(args, req.Limit, req.Offset)
	rows, err := d.db.QueryContext(ctx, query, args...)
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

// UpdateCell updates exactly one row identified by its declared primary key.
func (d *Driver) UpdateCell(ctx context.Context, request db.CellUpdateRequest) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	query, args, err := mysqlUpdateCellStatement(request)
	if err != nil {
		return err
	}
	existsQuery, existsArgs, err := mysqlCellExistsStatement(request)
	if err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var found int
	if err := tx.QueryRowContext(ctx, existsQuery, existsArgs...).Scan(&found); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("cell update affected 0 rows, want 1")
		}
		return err
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 1 {
		return fmt.Errorf("cell update affected %d rows, want 1", affected)
	}
	return tx.Commit()
}

func mysqlUpdateCellStatement(request db.CellUpdateRequest) (string, []any, error) {
	if err := request.Validate(); err != nil {
		return "", nil, err
	}
	args := make([]any, 0, len(request.PrimaryKey)+1)
	args = append(args, request.Value)
	where := make([]string, 0, len(request.PrimaryKey))
	for _, key := range request.PrimaryKey {
		where = append(where, quoteIdent(key.Column)+" = ?")
		args = append(args, key.Value)
	}
	query := "update " + qualifiedName(request.Table.Schema, request.Table.Name) + " set " + quoteIdent(request.Column) + " = ? where " + strings.Join(where, " and ")
	return query, args, nil
}

func mysqlCellExistsStatement(request db.CellUpdateRequest) (string, []any, error) {
	if err := request.Validate(); err != nil {
		return "", nil, err
	}
	args := make([]any, 0, len(request.PrimaryKey))
	where := make([]string, 0, len(request.PrimaryKey))
	for _, key := range request.PrimaryKey {
		where = append(where, quoteIdent(key.Column)+" = ?")
		args = append(args, key.Value)
	}
	query := "select 1 from " + qualifiedName(request.Table.Schema, request.Table.Name) + " where " + strings.Join(where, " and ") + " for update"
	return query, args, nil
}

func (d *Driver) OpenTableRowStream(ctx context.Context, req db.TableRowStreamRequest) ([]db.Column, db.RowStream, error) {
	if d.db == nil {
		return nil, nil, fmt.Errorf("not connected")
	}
	columns, err := d.columnMetadata(ctx, req.Table.Schema, req.Table.Name)
	if err != nil {
		return nil, nil, err
	}
	where, args, err := mysqlBrowseWhere(req.Filters)
	if err != nil {
		return nil, nil, err
	}
	query := fmt.Sprintf("select * from %s", qualifiedName(req.Table.Schema, req.Table.Name)) + where
	if orderBy := sortOrder(columns, req.Sort); orderBy != "" {
		query += " order by " + orderBy
	}
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	return columns, &rowStream{rows: rows, values: values, scanTargets: scanTargets}, nil
}

type rowStream struct {
	mu          sync.Mutex
	rows        *sql.Rows
	values      []any
	scanTargets []any
	closed      bool
}

func (s *rowStream) Next() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, io.EOF
	}
	if !s.rows.Next() {
		err := s.rows.Err()
		_ = s.close()
		if err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	if err := s.rows.Scan(s.scanTargets...); err != nil {
		_ = s.close()
		return nil, err
	}
	row := make([]string, len(s.values))
	for i, value := range s.values {
		row[i] = formatValue(value)
	}
	return row, nil
}

func (s *rowStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close()
}

func (s *rowStream) close() error {
	if !s.closed {
		s.closed = true
		return s.rows.Close()
	}
	return nil
}

func (d *Driver) CountBrowseRows(ctx context.Context, req db.BrowseRequest) (int64, error) {
	if d.db == nil {
		return 0, fmt.Errorf("not connected")
	}
	where, args, err := mysqlBrowseWhere(req.Filters)
	if err != nil {
		return 0, err
	}
	var count int64
	err = d.db.QueryRowContext(ctx, fmt.Sprintf("select count(*) from %s", qualifiedName(req.Table.Schema, req.Table.Name))+where, args...).Scan(&count)
	return count, err
}

func mysqlBrowseWhere(filters []db.RowFilter) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	parts := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	for _, filter := range filters {
		column := quoteIdent(filter.Column)
		switch filter.Operator {
		case db.FilterEqual:
			parts = append(parts, column+" = ?")
			args = append(args, filter.Value)
		case db.FilterContains:
			parts = append(parts, "lower(cast("+column+" as char)) like lower(?) escape '\\\\'")
			args = append(args, "%"+escapeLikeLiteral(filter.Value)+"%")
		case db.FilterLike:
			parts = append(parts, "cast("+column+" as char) like ?")
			args = append(args, filter.Value)
		case db.FilterGreater:
			parts = append(parts, column+" > ?")
			args = append(args, filter.Value)
		case db.FilterLess:
			parts = append(parts, column+" < ?")
			args = append(args, filter.Value)
		case db.FilterGreaterOrEqual:
			parts = append(parts, column+" >= ?")
			args = append(args, filter.Value)
		case db.FilterLessOrEqual:
			parts = append(parts, column+" <= ?")
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
	return db.Result{Columns: columns, Rows: resultRows, DurationMs: time.Since(started).Milliseconds(), Truncated: truncated}, nil
}

func (d *Driver) OpenQueryRowStream(ctx context.Context, query db.Query) (db.QueryRowStream, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	started := time.Now()
	if !returnsRows(query.SQL) {
		result, err := d.db.ExecContext(ctx, query.SQL, query.Args...)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			affected = 0
		}
		return completedQueryRowStream{rowsAffected: affected, durationMs: time.Since(started).Milliseconds()}, nil
	}
	rows, err := d.db.QueryContext(ctx, query.SQL, query.Args...)
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, err
	}
	values := make([]any, len(columns))
	scanTargets := make([]any, len(columns))
	for i := range values {
		scanTargets[i] = &values[i]
	}
	return &queryRowStream{rows: rows, columns: columns, values: values, scanTargets: scanTargets, started: started}, nil
}

type completedQueryRowStream struct {
	rowsAffected int64
	durationMs   int64
}

func (completedQueryRowStream) Next() ([]string, error) { return nil, io.EOF }
func (completedQueryRowStream) Close() error            { return nil }
func (completedQueryRowStream) Columns() []string       { return nil }
func (s completedQueryRowStream) RowsAffected() int64   { return s.rowsAffected }
func (s completedQueryRowStream) DurationMs() int64     { return s.durationMs }

type queryRowStream struct {
	mu          sync.Mutex
	rows        *sql.Rows
	columns     []string
	values      []any
	scanTargets []any
	started     time.Time
	closed      bool
}

func (s *queryRowStream) Next() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, io.EOF
	}
	if !s.rows.Next() {
		err := s.rows.Err()
		_ = s.close()
		if err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	if err := s.rows.Scan(s.scanTargets...); err != nil {
		_ = s.close()
		return nil, err
	}
	row := make([]string, len(s.values))
	for i, value := range s.values {
		row[i] = formatValue(value)
	}
	return row, nil
}

func (s *queryRowStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close()
}

func (s *queryRowStream) close() error {
	if !s.closed {
		s.closed = true
		return s.rows.Close()
	}
	return nil
}

func (s *queryRowStream) Columns() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.columns...)
}
func (*queryRowStream) RowsAffected() int64 { return 0 }
func (s *queryRowStream) DurationMs() int64 { return time.Since(s.started).Milliseconds() }

func (d *Driver) columnMetadata(ctx context.Context, schema, table string) ([]db.Column, error) {
	if d.db == nil {
		return nil, fmt.Errorf("not connected")
	}
	rows, err := d.db.QueryContext(ctx, `
		select column_name, column_type, is_nullable, column_default, column_key = 'PRI'
		from information_schema.columns
		where table_schema = ? and table_name = ?
		order by ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []db.Column
	for rows.Next() {
		var name, dataType, nullable string
		var defaultValue sql.NullString
		var primary bool
		if err := rows.Scan(&name, &dataType, &nullable, &defaultValue, &primary); err != nil {
			return nil, err
		}
		column := db.Column{Name: name, DataType: dataType, Nullable: nullable == "YES", IsPrimary: primary}
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
	rows, err := d.db.QueryContext(ctx, `
		select column_name, referenced_table_schema, referenced_table_name, referenced_column_name
		from information_schema.key_column_usage
		where table_schema = ?
		  and table_name = ?
		  and referenced_table_name is not null`, schema, table)
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

func rowOrder(columns []db.Column) string {
	var primary []string
	for _, column := range columns {
		if column.IsPrimary {
			primary = append(primary, quoteIdent(column.Name))
		}
	}
	return strings.Join(primary, ", ")
}

func sortOrder(columns []db.Column, sort db.SortSpec) string {
	if sort.Column == "" {
		return rowOrder(columns)
	}
	for _, column := range columns {
		if column.Name == sort.Column {
			direction := "asc"
			if sort.Descending {
				direction = "desc"
			}
			return quoteIdent(column.Name) + " " + direction
		}
	}
	return ""
}

// SortedQuery wraps a read-only SQL result with MySQL-safe ordering.
func (*Driver) SortedQuery(query db.Query, sort db.SortSpec) (db.Query, error) {
	if sort.Column == "" {
		return query, nil
	}
	statement := strings.TrimSuffix(strings.TrimSpace(query.SQL), ";")
	if statement == "" {
		return db.Query{}, fmt.Errorf("SQL query is required")
	}
	direction := "asc"
	if sort.Descending {
		direction = "desc"
	}
	return db.Query{SQL: "select * from (" + statement + ") as sqvue_query order by " + quoteIdent(sort.Column) + " " + direction, Args: query.Args}, nil
}

func returnsRows(query string) bool {
	statement := strings.ToLower(stripLeadingSQLComments(query))
	switch sqlKeyword(statement) {
	case "select", "with", "show", "describe", "desc", "explain", "values":
		return true
	}
	return false
}

func stripLeadingSQLComments(query string) string {
	statement := strings.TrimSpace(query)
	for {
		switch {
		case strings.HasPrefix(statement, "--"):
			if end := strings.IndexByte(statement, '\n'); end >= 0 {
				statement = strings.TrimSpace(statement[end+1:])
			} else {
				return ""
			}
		case strings.HasPrefix(statement, "/*"):
			end := strings.Index(statement[2:], "*/")
			if end < 0 {
				return ""
			}
			statement = strings.TrimSpace(statement[end+4:])
		default:
			return statement
		}
	}
}

func sqlKeyword(statement string) string {
	for i, r := range statement {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return statement[:i]
		}
	}
	return statement
}

func qualifiedName(schema, name string) string {
	return quoteIdent(schema) + "." + quoteIdent(name)
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
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
	db.Register(db.DbTypeMySQL, func() db.Driver { return New() })
}
