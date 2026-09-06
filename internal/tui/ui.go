package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Update dispatches Bubble Tea messages to the handler that owns their state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m.handleLoadMessage(msg)
	}
}

func (m Model) handleLoadMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case schemasLoadedMsg:
		return m.handleSchemasLoaded(msg)
	case tablesLoadedMsg:
		return m.handleTablesLoaded(msg)
	case rowsLoadedMsg:
		return m.handleRowsLoaded(msg)
	case tableStreamRowsLoadedMsg:
		return m.handleTableStreamRowsLoaded(msg)
	case descriptionsLoadedMsg:
		return m.handleDescriptionsLoaded(msg)
	case countLoadedMsg:
		return m.handleCountLoaded(msg)
	case queryLoadedMsg:
		return m.handleQueryLoaded(msg)
	case queryStreamRowsLoadedMsg:
		return m.handleQueryStreamRowsLoaded(msg)
	default:
		return m.handleCompletionMessage(msg)
	}
}

func (m Model) handleCompletionMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case clipboardWrittenMsg:
		if msg.copyStatusID != m.copyStatusID {
			return m, nil
		}
		if msg.err != nil {
			return m.fail("copy failed", msg.err)
		}
		m.status, m.copyStatusKind, m.lastErr = "copied "+msg.kind, msg.kind, nil
		return m, clearCopyStatusCmd(msg.kind, msg.copyStatusID)
	case copyStatusClearedMsg:
		if msg.copyStatusID != m.copyStatusID || msg.kind != m.copyStatusKind || m.status != "copied "+msg.kind {
			return m, nil
		}
		m.restoreBrowseStatus()
	case exportedMsg:
		if msg.exportID != m.exportID {
			return m, nil
		}
		if msg.err != nil {
			return m.fail(string(msg.format)+" export failed", msg.err)
		}
		m.status, m.lastErr = fmt.Sprintf("exported %d rows to %s", msg.rows, msg.path), nil
	case backupCompletedMsg:
		if msg.backupID != m.backupID {
			return m, nil
		}
		if msg.err != nil {
			return m.fail("database backup failed", msg.err)
		}
		m.status, m.lastErr = fmt.Sprintf("database backed up to %s", msg.path), nil
	case cellUpdatedMsg:
		if msg.updateID != m.updateID {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			return m.fail("cell update failed", msg.err)
		}
		m.cellEditRefreshPending = true
		return m.startLoadRows()
	case profileConnectedMsg:
		return m.handleProfileConnected(msg)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	oldSize := m.pageSize
	m.width, m.height = msg.Width, msg.Height
	m.filterInput.Width = inputWidth(msg.Width, m.filterInput.Prompt)
	m.browseFilterInput.Width = inputWidth(msg.Width, m.browseFilterInput.Prompt)
	m.sqlInput.SetWidth(max(1, msg.Width-2))
	m.sqlInput.SetHeight(min(8, max(3, msg.Height/3)))
	m.exportInput.Width = inputWidth(msg.Width, m.exportInput.Prompt)
	m.backupInput.Width = inputWidth(msg.Width, m.backupInput.Prompt)
	m.cellEditInput.Width = inputWidth(msg.Width, m.cellEditInput.Prompt)
	m.queryNameInput.Width = inputWidth(msg.Width, m.queryNameInput.Prompt)
	if msg.Height > 0 {
		m.pageSize = m.computedPageSize()
	}
	if m.pageSize != oldSize && m.queryActive {
		if m.queryStreaming {
			m.closeQueryStream()
			return m.startLoadQueryRows()
		}
		m.setQueryPage()
		return m, nil
	}
	if m.pageSize != oldSize && len(m.tables) > 0 {
		m.closeTableStream()
		return m.startLoad()
	}
	return m, nil
}

func inputWidth(terminalWidth int, prompt string) int {
	return max(1, terminalWidth-ansi.StringWidth(prompt))
}
func (m Model) computedPageSize() int { return clamp(m.height-reservedRows, 1, maxPageSize) }

// handleOverlayKey routes keys to the active modal before normal navigation.
func (m Model) handleOverlayKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch m.activeOverlay {
	case overlayNone:
		return m, nil, false
	case overlayHelp:
		m.activeOverlay = overlayNone
		return m, nil, true
	case overlaySQL:
		m, cmd := m.handleSQLKey(msg)
		return m, cmd, true
	case overlaySaveQueryName:
		m, cmd := m.handleSaveQueryNameKey(msg)
		return m, cmd, true
	case overlaySavedQueries:
		m, cmd := m.handleSavedQueriesKey(msg)
		return m, cmd, true
	case overlayRenameQuery:
		m, cmd := m.handleRenameQueryKey(msg)
		return m, cmd, true
	case overlayDeleteQueryConfirm:
		m, cmd := m.handleDeleteQueryConfirmKey(msg)
		return m, cmd, true
	case overlayExport:
		m, cmd := m.handleExportKey(msg)
		return m, cmd, true
	case overlayBackupScope:
		m, cmd := m.handleBackupScopeKey(msg)
		return m, cmd, true
	case overlayBackupPath:
		m, cmd := m.handleBackupPathKey(msg)
		return m, cmd, true
	case overlayCellEdit:
		m, cmd := m.handleCellEditKey(msg)
		return m, cmd, true
	case overlayCellEditConfirm:
		m, cmd := m.handleCellEditConfirmKey(msg)
		return m, cmd, true
	case overlayColumnPicker:
		m, cmd := m.handleColumnsKey(msg)
		return m, cmd, true
	case overlayRowDetail:
		m, cmd := m.handleRowDetailKey(msg)
		return m, cmd, true
	case overlayFilter:
		m, cmd := m.handleFilterKey(msg)
		return m, cmd, true
	case overlayBrowseFilter:
		m, cmd := m.handleBrowseFilterKey(msg)
		return m, cmd, true
	case overlayBrowseFilterOperator:
		m, cmd := m.handleBrowseFilterOperatorKey(msg)
		return m, cmd, true
	case overlaySchemaPicker:
		m, cmd := m.handleSchemaKey(msg)
		return m, cmd, true
	case overlayProfilePicker:
		m, cmd := m.handleProfilePickerKey(msg)
		return m, cmd, true
	}
	return m, nil, false
}

func (m Model) handleRowDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || key.Matches(msg, m.keys.Confirm):
		m.activeOverlay = overlayNone
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.detailScroll = min(m.detailScroll+1, m.maxDetailScroll())
	case key.Matches(msg, m.keys.Up):
		m.detailScroll = max(0, m.detailScroll-1)
	}
	return m, nil
}

func (m Model) View() string { return Render(m) }
