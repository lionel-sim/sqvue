package tui

import (
	"strings"
	"testing"

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
