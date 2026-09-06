package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func TestBackupShortcutOpensScopePicker(t *testing.T) {
	m := New(Options{Client: &fakeDriver{}})

	m, _ = update(m, keyMsg("B"))
	if m.activeOverlay != overlayBackupScope {
		t.Fatalf("active overlay = %v, want backup scope picker", m.activeOverlay)
	}
	if m.status != "select a backup scope" {
		t.Fatalf("status = %q, want backup prompt", m.status)
	}

	m, _ = update(m, keyMsg("esc"))
	if m.activeOverlay != overlayNone {
		t.Fatalf("active overlay = %v, want none after escape", m.activeOverlay)
	}
}

func TestBackupScopeConfirmPromptsForDestination(t *testing.T) {
	m := New(Options{Client: &fakeDriver{}, ExportDirectory: "/backups"})

	m, _ = update(m, keyMsg("B"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.activeOverlay != overlayBackupPath {
		t.Fatalf("active overlay = %v, want backup path prompt", m.activeOverlay)
	}
	if !strings.HasPrefix(m.backupInput.Value(), "/backups/sqvue-backup-") || !strings.HasSuffix(m.backupInput.Value(), ".sql") {
		t.Fatalf("default backup path = %q, want SQL backup under /backups", m.backupInput.Value())
	}
}

func TestDefaultBackupPathUsesSelectedTableAndDriverExtension(t *testing.T) {
	m := New(Options{Client: &fakeDriver{backupCaps: db.BackupCapabilities{Database: true, Table: true, FileExtension: "db"}}, ExportDirectory: "/backups"})
	m.tables = []db.Table{{Schema: "main", Name: "order items"}}
	m.backupScopeCursor = 1

	got := m.defaultBackupPath(time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC))
	if want := "/backups/sqvue-backup-main-order_items-20260906-010203.db"; got != want {
		t.Fatalf("defaultBackupPath() = %q, want %q", got, want)
	}
}

func TestBackupPathRejectsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.sql")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Client: &fakeDriver{}})
	m, _ = update(m, keyMsg("B"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m.backupInput.SetValue(path)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("existing backup path should not start a backup")
	}
	if !strings.Contains(m.status, "backup file already exists") {
		t.Fatalf("status = %q, want existing-file error", m.status)
	}
}

func TestBackupPathStartsAsyncBackupAndReportsCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.sql")
	driver := &fakeDriver{}
	m := New(Options{Client: driver, Timeout: time.Second})
	m, _ = update(m, keyMsg("B"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m.backupInput.SetValue(path)

	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.status != "backing up database..." {
		t.Fatalf("status = %q, want backup progress", m.status)
	}
	m, _ = update(m, runCmd(cmd))
	if driver.lastBackup.Path != path || driver.lastBackup.Scope != db.BackupScopeDatabase {
		t.Fatalf("backup request = %#v, want database backup at %q", driver.lastBackup, path)
	}
	if m.status != "database backed up to "+path {
		t.Fatalf("status = %q, want completion", m.status)
	}
}

func TestBackupScopePickerRendersAsModal(t *testing.T) {
	m := testModel()
	m.width, m.height = 100, 20
	m.activeOverlay = overlayBackupScope

	out := m.View()
	if !strings.Contains(out, "Back up database") {
		t.Fatalf("backup scope dialog missing heading: %q", out)
	}
}

func TestBackupScopePickerOffersSupportedScopes(t *testing.T) {
	m := New(Options{Client: &fakeDriver{backupCaps: db.BackupCapabilities{Database: true, Schema: true, Table: true}}})
	m.schemas = []db.Schema{{Name: "public"}}
	m.tables = []db.Table{{Schema: "public", Name: "orders"}}

	options := m.backupScopeOptions()
	if len(options) != 3 {
		t.Fatalf("backup scope options = %#v, want database, schema, and table", options)
	}
	if options[0].scope != db.BackupScopeDatabase || options[0].label != "Entire database" {
		t.Fatalf("default backup scope = %#v, want entire database", options[0])
	}

	m, _ = update(m, keyMsg("B"))
	m, _ = update(m, keyMsg("j"))
	if m.backupScopeCursor != 1 {
		t.Fatalf("backup scope cursor = %d, want 1 after j", m.backupScopeCursor)
	}
}
