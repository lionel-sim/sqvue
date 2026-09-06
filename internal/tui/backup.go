package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	case key.Matches(msg, m.keys.Confirm):
		m.backupInput.SetValue(m.defaultBackupPath(time.Now()))
		m.backupInput.Focus()
		m.activeOverlay = overlayBackupPath
	case key.Matches(msg, m.keys.Down):
		m.backupScopeCursor = min(m.backupScopeCursor+1, len(options)-1)
	case key.Matches(msg, m.keys.Up):
		m.backupScopeCursor = max(0, m.backupScopeCursor-1)
	}
	return m, nil
}

func (m Model) handleBackupPathKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.activeOverlay = overlayNone
		m.backupInput.Blur()
		m.restoreBrowseStatus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		request, err := m.backupRequest(strings.TrimSpace(m.backupInput.Value()))
		if err != nil {
			m.status = "backup failed: " + err.Error()
			return m, nil
		}
		driver := m.backupDriver()
		m.activeOverlay = overlayNone
		m.backupInput.Blur()
		m.backupID++
		m.status = "backing up database..."
		return m, runBackupCmd(driver, m.timeout, m.backupID, request)
	}
	var cmd tea.Cmd
	m.backupInput, cmd = m.backupInput.Update(msg)
	return m, cmd
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
	content.WriteString("\n" + theme.Muted.Render("Use j/k to choose, Enter to continue, Esc to cancel."))
	panelLines := strings.Split(theme.Dialog.Width(46).Render(content.String()), "\n")
	return titledDialog(panelLines, ansi.StringWidth(panelLines[0]), "Back up database")
}

func renderBackupPathModal(m Model) string {
	width := backupPathDialogWidth(m)
	input := m.backupInput
	input.Width = max(1, width-ansi.StringWidth(input.Prompt)-4)
	content := input.View() + "\n\n" + theme.Muted.Render("Enter starts the backup. Esc cancels.")
	panelLines := strings.Split(theme.Dialog.Width(width).Render(content), "\n")
	return titledDialog(panelLines, ansi.StringWidth(panelLines[0]), "Save backup")
}

func backupPathDialogWidth(m Model) int {
	if m.width <= 0 {
		return 80
	}
	return min(80, max(20, m.width-6))
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

func (m Model) selectedBackupScope() (backupScopeOption, error) {
	options := m.backupScopeOptions()
	if m.backupScopeCursor < 0 || m.backupScopeCursor >= len(options) {
		return backupScopeOption{}, fmt.Errorf("no backup scope selected")
	}
	return options[m.backupScopeCursor], nil
}

func (m Model) defaultBackupPath(now time.Time) string {
	directory := m.exportDirectory
	if directory == "" {
		directory = "."
	}
	name := "sqvue-backup"
	if option, err := m.selectedBackupScope(); err == nil {
		switch option.scope {
		case db.BackupScopeSchema:
			name += "-" + safeExportName(m.currentSchema())
		case db.BackupScopeTable:
			if table := m.currentTable(); table != nil {
				name += "-" + safeExportName(table.Schema) + "-" + safeExportName(table.Name)
			}
		}
	}
	extension := "backup"
	if driver := m.backupDriver(); driver != nil && driver.BackupCapabilities().FileExtension != "" {
		extension = strings.TrimPrefix(driver.BackupCapabilities().FileExtension, ".")
	}
	return filepath.Join(directory, name+"-"+now.Format("20060102-150405")+"."+extension)
}

func (m Model) backupRequest(path string) (db.BackupRequest, error) {
	option, err := m.selectedBackupScope()
	if err != nil {
		return db.BackupRequest{}, err
	}
	request := db.BackupRequest{Scope: option.scope, Path: path, Schema: m.currentSchema()}
	if table := m.currentTable(); table != nil {
		request.Table = *table
	}
	if err := request.Validate(); err != nil {
		return db.BackupRequest{}, err
	}
	if _, err := os.Lstat(path); err == nil {
		return db.BackupRequest{}, fmt.Errorf("backup file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return db.BackupRequest{}, fmt.Errorf("inspect backup path: %w", err)
	}
	return request, nil
}

func runBackupCmd(driver db.BackupDriver, timeout time.Duration, backupID uint64, request db.BackupRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := driver.Backup(ctx, request)
		return backupCompletedMsg{backupID: backupID, path: request.Path, err: err}
	}
}
