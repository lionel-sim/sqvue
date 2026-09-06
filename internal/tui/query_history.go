package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSQLHistoryKey(msg tea.KeyMsg) (Model, bool) {
	if m.queryStore == nil {
		return m, false
	}
	history := m.queryStore.History(m.profileName)
	if len(history) == 0 {
		return m, false
	}
	switch {
	case key.Matches(msg, m.keys.HistoryPrev):
		if m.historyIndex == -1 {
			m.historyDraft, m.historyIndex = m.sqlInput.Value(), len(history)
		}
		m.historyIndex = max(0, m.historyIndex-1)
		m.sqlInput.SetValue(history[m.historyIndex])
		return m, true
	case key.Matches(msg, m.keys.HistoryNext):
		if m.historyIndex == -1 {
			return m, false
		}
		m.historyIndex++
		if m.historyIndex >= len(history) {
			m.historyIndex = -1
			m.sqlInput.SetValue(m.historyDraft)
			return m, true
		}
		m.sqlInput.SetValue(history[m.historyIndex])
		return m, true
	}
	return m, false
}

func (m Model) beginSaveQuery() (Model, tea.Cmd) {
	if strings.TrimSpace(m.sqlInput.Value()) == "" {
		m.status = "enter SQL before saving a query"
		return m, nil
	}
	m.activeOverlay = overlaySaveQueryName
	m.queryNameInput.SetValue("")
	m.queryNameInput.Focus()
	m.status = "name saved query"
	return m, nil
}

func (m Model) openSavedQueries() (Model, tea.Cmd) {
	if m.queryStore == nil {
		m.status = "saved queries are unavailable without a query store"
		return m, nil
	}
	m.savedQueryNames = m.queryStore.QueryNames(m.profileName)
	m.savedQueryCursor = 0
	m.activeOverlay = overlaySavedQueries
	m.status = "select a saved query"
	return m, nil
}

func (m Model) handleSaveQueryNameKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlaySQL
		m.queryNameInput.Blur()
		m.sqlInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		if m.queryStore == nil {
			m.status = "saved queries are unavailable without a query store"
			return m, nil
		}
		if err := m.queryStore.SaveQuery(m.profileName, m.queryNameInput.Value(), m.sqlInput.Value()); err != nil {
			m.status, m.lastErr = "save query failed: "+err.Error(), err
			return m, nil
		}
		name := strings.TrimSpace(m.queryNameInput.Value())
		m.activeOverlay, m.lastErr = overlaySQL, nil
		m.queryNameInput.Blur()
		m.sqlInput.Focus()
		m.status = fmt.Sprintf("saved query %q", name)
		return m, nil
	}
	var cmd tea.Cmd
	m.queryNameInput, cmd = m.queryNameInput.Update(msg)
	return m, cmd
}

func (m Model) handleSavedQueriesKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlaySQL
		m.sqlInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.savedQueryCursor = min(m.savedQueryCursor+1, len(m.savedQueryNames)-1)
	case key.Matches(msg, m.keys.Up):
		m.savedQueryCursor = max(0, m.savedQueryCursor-1)
	case key.Matches(msg, m.keys.Confirm):
		if len(m.savedQueryNames) == 0 {
			return m, nil
		}
		name := m.savedQueryNames[m.savedQueryCursor]
		query, ok := m.queryStore.Query(m.profileName, name)
		if !ok {
			m.status = fmt.Sprintf("saved query %q no longer exists", name)
			return m.openSavedQueries()
		}
		m.historyIndex = -1
		m.status = fmt.Sprintf("running saved query %q...", name)
		return m.runSQL(query)
	case msg.String() == "r" && len(m.savedQueryNames) > 0:
		m.savedQueryRename = m.savedQueryNames[m.savedQueryCursor]
		m.queryNameInput.SetValue(m.savedQueryRename)
		m.queryNameInput.Focus()
		m.activeOverlay = overlayRenameQuery
		m.status = "rename saved query"
	case msg.String() == "d" && len(m.savedQueryNames) > 0:
		m.activeOverlay = overlayDeleteQueryConfirm
		m.status = "confirm saved-query deletion"
	}
	return m, nil
}

func (m Model) handleRenameQueryKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlaySavedQueries
		m.queryNameInput.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		if err := m.queryStore.RenameQuery(m.profileName, m.savedQueryRename, m.queryNameInput.Value()); err != nil {
			m.status, m.lastErr = "rename query failed: "+err.Error(), err
			return m, nil
		}
		m.queryNameInput.Blur()
		m.status, m.lastErr = "renamed saved query", nil
		return m.openSavedQueries()
	}
	var cmd tea.Cmd
	m.queryNameInput, cmd = m.queryNameInput.Update(msg)
	return m, cmd
}

func (m Model) handleDeleteQueryConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.activeOverlay = overlaySavedQueries
		return m, nil
	}
	if !key.Matches(msg, m.keys.Confirm) {
		return m, nil
	}
	name := m.savedQueryNames[m.savedQueryCursor]
	if err := m.queryStore.DeleteQuery(m.profileName, name); err != nil {
		m.status, m.lastErr = "delete query failed: "+err.Error(), err
		return m, nil
	}
	m.status, m.lastErr = fmt.Sprintf("deleted saved query %q", name), nil
	return m.openSavedQueries()
}
