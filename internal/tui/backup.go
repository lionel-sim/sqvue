package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
	"sqvue/internal/theme"
)

func (m Model) backupDriver() db.BackupDriver {
	driver, _ := m.client.(db.BackupDriver)
	return driver
}

func (m Model) beginBackup() (Model, tea.Cmd) {
	if m.backupDriver() == nil {
		m.status = "database backups are unavailable for this connection"
		return m, nil
	}
	m.activeOverlay = overlayBackupScope
	m.status = "select a backup scope"
	return m, nil
}

func (m Model) handleBackupScopeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.restoreBrowseStatus()
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	}
	return m, nil
}

func renderBackupScopeModal(m Model) string {
	content := theme.Muted.Render("Backup scope options will appear here.") + "\n\n" +
		theme.Muted.Render("Press Esc to cancel.")
	panelLines := strings.Split(theme.Dialog.Width(46).Render(content), "\n")
	return titledDialog(panelLines, ansi.StringWidth(panelLines[0]), "Back up database")
}
