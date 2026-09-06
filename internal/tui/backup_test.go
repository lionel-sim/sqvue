package tui

import (
	"strings"
	"testing"
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
