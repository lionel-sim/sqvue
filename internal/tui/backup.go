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
	if len(m.backupScopeOptions()) == 0 {
		m.status = "database backups are unavailable for this connection"
		return m, nil
	}
	m.activeOverlay, m.backupScopeCursor = overlayBackupScope, 0
	m.status = "select a backup scope"
	return m, nil
}

func (m Model) handleBackupScopeKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	options := m.backupScopeOptions()
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.restoreBrowseStatus()
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.backupScopeCursor = min(m.backupScopeCursor+1, len(options)-1)
	case key.Matches(msg, m.keys.Up):
		m.backupScopeCursor = max(0, m.backupScopeCursor-1)
	}
	return m, nil
}

func renderBackupScopeModal(m Model) string {
	var content strings.Builder
	content.WriteString("Choose what to back up:\n\n")
	for i, option := range m.backupScopeOptions() {
		line := "  " + option.label
		if i == m.backupScopeCursor {
			line = theme.Selected.Render("→ " + option.label)
		}
		content.WriteString(line + "\n")
	}
	content.WriteString("\n" + theme.Muted.Render("Use j/k to choose, Esc to cancel."))
	panelLines := strings.Split(theme.Dialog.Width(46).Render(content.String()), "\n")
	return titledDialog(panelLines, ansi.StringWidth(panelLines[0]), "Back up database")
}

type backupScopeOption struct {
	scope db.BackupScope
	label string
}

func (m Model) backupScopeOptions() []backupScopeOption {
	driver := m.backupDriver()
	if driver == nil {
		return nil
	}
	capabilities := driver.BackupCapabilities()
	options := make([]backupScopeOption, 0, 3)
	if capabilities.Database {
		options = append(options, backupScopeOption{scope: db.BackupScopeDatabase, label: "Entire database"})
	}
	if capabilities.Schema && m.currentSchema() != "" {
		options = append(options, backupScopeOption{scope: db.BackupScopeSchema, label: "Current schema"})
	}
	if capabilities.Table && m.currentTable() != nil {
		options = append(options, backupScopeOption{scope: db.BackupScopeTable, label: "Current table"})
	}
	return options
}
