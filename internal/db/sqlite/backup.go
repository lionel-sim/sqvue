package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sqvue/internal/db"
)

const sqliteBackupSchema = "sqvue_backup"

var (
	createTablePrefix   = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?`)
	createIndexPrefix   = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?`)
	createTriggerPrefix = regexp.MustCompile(`(?is)^\s*CREATE\s+TRIGGER\s+(?:IF\s+NOT\s+EXISTS\s+)?`)
)

func (d *Driver) BackupCapabilities() db.BackupCapabilities {
	return db.BackupCapabilities{Database: true, Table: true, FileExtension: "db"}
}

// Backup creates a consistent SQLite database copy or a standalone table
// backup. Table backups include the table's schema, rows, indexes, and
// triggers, but cannot include other tables referenced by foreign keys.
func (d *Driver) Backup(ctx context.Context, request db.BackupRequest) error {
	if d.db == nil {
		return fmt.Errorf("not connected")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Scope != db.BackupScopeDatabase && request.Scope != db.BackupScopeTable {
		return fmt.Errorf("SQLite does not support %s backups", request.Scope)
	}
	if err := os.MkdirAll(filepath.Dir(request.Path), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if err := ensureSQLiteBackupPath(request.Path); err != nil {
		return err
	}
	if request.Scope == db.BackupScopeDatabase {
		if _, err := d.db.ExecContext(ctx, "vacuum into ?", request.Path); err != nil {
			return fmt.Errorf("vacuum database into backup: %w", err)
		}
		return nil
	}
	return d.backupTable(ctx, request)
}

func ensureSQLiteBackupPath(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("backup file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect backup path: %w", err)
	}
	return nil
}

func (d *Driver) backupTable(ctx context.Context, request db.BackupRequest) (returnErr error) {
	file, err := os.OpenFile(request.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("backup file already exists")
		}
		return fmt.Errorf("create backup file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close backup file: %w", err)
	}
	succeeded := false
	defer func() {
		if !succeeded {
			_ = os.Remove(request.Path)
		}
	}()

	connection, err := d.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open SQLite backup connection: %w", err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, "attach database ? as "+quoteIdent(sqliteBackupSchema), request.Path); err != nil {
		return fmt.Errorf("attach backup database: %w", err)
	}
	defer func() {
		_, _ = connection.ExecContext(context.Background(), "detach database "+quoteIdent(sqliteBackupSchema))
	}()

	tableDDL, err := sqliteTableDDL(ctx, connection, request.Table)
	if err != nil {
		return err
	}
	createTable, err := qualifySQLiteDDL(tableDDL, createTablePrefix, sqliteBackupSchema)
	if err != nil {
		return fmt.Errorf("prepare table schema: %w", err)
	}
	if _, err := connection.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("create backup table: %w", err)
	}
	sourceTable := qualifiedName(request.Table.Schema, request.Table.Name)
	targetTable := qualifiedName(sqliteBackupSchema, request.Table.Name)
	if _, err := connection.ExecContext(ctx, "insert into "+targetTable+" select * from "+sourceTable); err != nil {
		return fmt.Errorf("copy table rows: %w", err)
	}
	objects, err := sqliteTableObjects(ctx, connection, request.Table)
	if err != nil {
		return err
	}
	for _, object := range objects {
		prefix := createIndexPrefix
		if object.kind == "trigger" {
			prefix = createTriggerPrefix
		}
		statement, err := qualifySQLiteDDL(object.sql, prefix, sqliteBackupSchema)
		if err != nil {
			return fmt.Errorf("prepare %s schema: %w", object.kind, err)
		}
		if _, err := connection.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create backup %s: %w", object.kind, err)
		}
	}
	succeeded = true
	return nil
}

func sqliteTableDDL(ctx context.Context, connection interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, table db.Table) (string, error) {
	var statement string
	query := "select sql from " + quoteIdent(table.Schema) + ".sqlite_master where type = 'table' and name = ?"
	if err := connection.QueryRowContext(ctx, query, table.Name).Scan(&statement); err != nil {
		return "", fmt.Errorf("load table schema: %w", err)
	}
	if statement == "" {
		return "", fmt.Errorf("table %s has no SQL schema", table)
	}
	return statement, nil
}

type sqliteObject struct {
	kind string
	sql  string
}

func sqliteTableObjects(ctx context.Context, connection interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, table db.Table) ([]sqliteObject, error) {
	query := "select type, sql from " + quoteIdent(table.Schema) + ".sqlite_master where tbl_name = ? and type in ('index', 'trigger') and sql is not null order by type, name"
	rows, err := connection.QueryContext(ctx, query, table.Name)
	if err != nil {
		return nil, fmt.Errorf("load table indexes and triggers: %w", err)
	}
	defer rows.Close()
	var objects []sqliteObject
	for rows.Next() {
		var object sqliteObject
		if err := rows.Scan(&object.kind, &object.sql); err != nil {
			return nil, fmt.Errorf("scan table object: %w", err)
		}
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read table objects: %w", err)
	}
	return objects, nil
}

func qualifySQLiteDDL(statement string, prefix *regexp.Regexp, schema string) (string, error) {
	location := prefix.FindStringIndex(statement)
	if location == nil {
		return "", fmt.Errorf("unsupported SQLite DDL")
	}
	nameEnd, err := sqliteIdentifierEnd(statement[location[1]:])
	if err != nil {
		return "", err
	}
	return statement[:location[1]] + quoteIdent(schema) + "." + statement[location[1]:location[1]+nameEnd] + statement[location[1]+nameEnd:], nil
}

func sqliteIdentifierEnd(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("missing object name")
	}
	switch value[0] {
	case '"', '`':
		for i := 1; i < len(value); i++ {
			if value[i] != value[0] {
				continue
			}
			if i+1 < len(value) && value[i+1] == value[0] {
				i++
				continue
			}
			return i + 1, nil
		}
		return 0, fmt.Errorf("unterminated quoted object name")
	case '[':
		if end := strings.IndexByte(value, ']'); end >= 0 {
			return end + 1, nil
		}
		return 0, fmt.Errorf("unterminated bracketed object name")
	default:
		end := strings.IndexFunc(value, func(r rune) bool { return r == '(' || r == ' ' || r == '\t' || r == '\n' || r == '\r' })
		if end < 0 {
			return len(value), nil
		}
		if end == 0 {
			return 0, fmt.Errorf("missing object name")
		}
		return end, nil
	}
}
