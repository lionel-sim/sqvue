package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"sqvue/internal/db"
)

func (d *Driver) BackupCapabilities() db.BackupCapabilities {
	return db.BackupCapabilities{Database: true, Table: true, FileExtension: "sql"}
}

// Backup writes a plain SQL pg_dump archive. The exclusive destination file
// ensures that a backup can never replace an existing artifact.
func (d *Driver) Backup(ctx context.Context, request db.BackupRequest) (returnErr error) {
	if d.pool == nil {
		return fmt.Errorf("not connected")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Scope != db.BackupScopeDatabase && request.Scope != db.BackupScopeTable {
		return fmt.Errorf("PostgreSQL does not support %s backups", request.Scope)
	}
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf("pg_dump is required for PostgreSQL backups: install PostgreSQL client tools and ensure pg_dump is on PATH")
	}
	if err := os.MkdirAll(filepath.Dir(request.Path), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	file, err := os.OpenFile(request.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("backup file already exists")
		}
		return fmt.Errorf("create backup file: %w", err)
	}
	succeeded := false
	defer func() {
		if closeErr := file.Close(); returnErr == nil && closeErr != nil {
			returnErr = fmt.Errorf("close backup file: %w", closeErr)
		}
		if !succeeded {
			_ = os.Remove(request.Path)
		}
	}()

	command, err := pgDumpCommand(ctx, d.backupConnect, request)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = file, &stderr
	if err := command.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("run pg_dump: %s", message)
		}
		return fmt.Errorf("run pg_dump: %w", err)
	}
	succeeded = true
	return nil
}

func pgDumpCommand(ctx context.Context, cfg db.ConnectConfig, request db.BackupRequest) (*exec.Cmd, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	pool, err := poolConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("parse PostgreSQL connection: %w", err)
	}
	connection := pool.ConnConfig
	args := []string{
		"--format=plain",
		"--host=" + connection.Host,
		"--port=" + strconv.Itoa(int(connection.Port)),
		"--username=" + connection.User,
		"--dbname=" + connection.Database,
	}
	if request.Scope == db.BackupScopeTable {
		args = append(args, "--table="+pgDumpTablePattern(request.Table))
	}
	command := exec.CommandContext(ctx, "pg_dump", args...)
	command.Env = append(os.Environ(),
		"PGHOST="+connection.Host,
		"PGPORT="+strconv.Itoa(int(connection.Port)),
		"PGUSER="+connection.User,
		"PGDATABASE="+connection.Database,
	)
	if connection.Password != "" {
		command.Env = append(command.Env, "PGPASSWORD="+connection.Password)
	}
	if cfg.SSLMode != "" {
		command.Env = append(command.Env, "PGSSLMODE="+cfg.SSLMode)
	}
	return command, nil
}

func pgDumpTablePattern(table db.Table) string {
	return `"` + strings.ReplaceAll(table.Schema, `"`, `""`) + `"."` + strings.ReplaceAll(table.Name, `"`, `""`) + `"`
}
