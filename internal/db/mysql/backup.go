package mysql

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"

	"sqvue/internal/db"
)

func (d *Driver) BackupCapabilities() db.BackupCapabilities {
	return db.BackupCapabilities{Database: true, Table: true, FileExtension: "sql"}
}

// Backup writes a plain SQL mysqldump archive. Credentials are passed through a
// temporary mode-600 option file rather than command-line arguments.
func (d *Driver) Backup(ctx context.Context, request db.BackupRequest) (returnErr error) {
	if d.db == nil || d.backupConfig == nil {
		return fmt.Errorf("not connected")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Scope != db.BackupScopeDatabase && request.Scope != db.BackupScopeTable {
		return fmt.Errorf("MySQL does not support %s backups", request.Scope)
	}
	if _, err := exec.LookPath("mysqldump"); err != nil {
		return fmt.Errorf("mysqldump is required for MySQL backups: install MySQL client tools and ensure mysqldump is on PATH")
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

	command, cleanup, err := mySQLDumpCommand(ctx, d.backupConfig, request)
	if err != nil {
		return err
	}
	defer cleanup()
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = file, &stderr
	if err := command.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return fmt.Errorf("run mysqldump: %s", message)
		}
		return fmt.Errorf("run mysqldump: %w", err)
	}
	succeeded = true
	return nil
}

func mySQLDumpCommand(ctx context.Context, cfg *mysqldriver.Config, request db.BackupRequest) (*exec.Cmd, func(), error) {
	optionFile, err := mySQLPasswordFile(cfg.Passwd)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.Remove(optionFile) }
	args, err := mySQLDumpArgs(cfg, request, optionFile)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return exec.CommandContext(ctx, "mysqldump", args...), cleanup, nil
}

func mySQLDumpArgs(cfg *mysqldriver.Config, request db.BackupRequest, optionFile string) ([]string, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if cfg.DBName == "" && request.Scope == db.BackupScopeDatabase {
		return nil, fmt.Errorf("MySQL database name is required for a database backup")
	}
	args := []string{"--defaults-extra-file=" + optionFile, "--single-transaction", "--routines", "--events", "--triggers", "--user=" + cfg.User}
	switch cfg.Net {
	case "", "tcp":
		host, port, err := net.SplitHostPort(cfg.Addr)
		if err != nil {
			return nil, fmt.Errorf("parse MySQL address: %w", err)
		}
		args = append(args, "--host="+host, "--port="+port)
	case "unix":
		args = append(args, "--socket="+cfg.Addr)
	default:
		return nil, fmt.Errorf("unsupported MySQL network %q", cfg.Net)
	}
	if request.Scope == db.BackupScopeDatabase {
		return append(args, "--databases", cfg.DBName), nil
	}
	return append(args, request.Table.Schema, request.Table.Name), nil
}

func mySQLPasswordFile(password string) (string, error) {
	file, err := os.CreateTemp("", "sqvue-mysqldump-*.cnf")
	if err != nil {
		return "", fmt.Errorf("create MySQL option file: %w", err)
	}
	path := file.Name()
	contents := "[client]\npassword=" + mySQLConfigValue(password) + "\n"
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write MySQL option file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close MySQL option file: %w", err)
	}
	return path, nil
}

func mySQLConfigValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + replacer.Replace(value) + `"`
}
