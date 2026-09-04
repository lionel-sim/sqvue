package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
	keymap "sqvue/internal/tui/components/keys"
)

func (m Model) handleColumnsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || key.Matches(msg, m.keys.Confirm):
		m.activeOverlay = overlayNone
		if m.queryActive {
			m.setQueryPage()
		} else if t := m.currentTable(); t != nil {
			m.setRowsStatus(*t)
		}
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.columnCursor = min(m.columnCursor+1, len(m.columns)-1)
	case key.Matches(msg, m.keys.Up):
		m.columnCursor = max(0, m.columnCursor-1)
	case key.Matches(msg, m.keys.Toggle):
		if m.visibleColumnCount() == 1 && m.visibleColumns[m.columnCursor] {
			m.status = "at least one column must remain visible"
			return m, nil
		}
		m.visibleColumns[m.columnCursor] = !m.visibleColumns[m.columnCursor]
	}
	m.columnScroll = keepInView(m.columnCursor, m.columnScroll, m.columnPickerHeight(), len(m.columns))
	return m, nil
}

func (m *Model) ensureVisibleColumns(key string) {
	if m.visibleColumnKey == key && len(m.visibleColumns) == len(m.columns) {
		return
	}
	m.visibleColumnKey, m.visibleColumns = key, make([]bool, len(m.columns))
	for i := range m.visibleColumns {
		m.visibleColumns[i] = true
	}
}
func (m Model) visibleColumnCount() int {
	count := 0
	for _, visible := range m.visibleColumns {
		if visible {
			count++
		}
	}
	return count
}
func (m Model) displayedColumnCount() int {
	if len(m.visibleColumns) == 0 {
		return len(m.columns)
	}
	return m.visibleColumnCount()
}
func (m Model) columnPickerHeight() int {
	if m.height <= 0 {
		return tableListHeight
	}
	return max(1, m.height-reservedRows-1)
}

func (m *Model) nextRequestID() uint64          { m.loadID++; return m.loadID }
func (m Model) isCurrent(requestID uint64) bool { return requestID == m.loadID }
func (m Model) fail(prefix string, err error) (Model, tea.Cmd) {
	m.status = fmt.Sprintf("%s: %v", prefix, err)
	m.lastErr = err
	return m, nil
}
func (m *Model) currentTable() *db.Table {
	if m.selected < 0 || m.selected >= len(m.tables) {
		return nil
	}
	return &m.tables[m.selected]
}

func (m Model) helpKeyMap() keymap.Map {
	keys := m.keys
	keys.Left.SetEnabled(m.focused)
	keys.Right.SetEnabled(m.focused)
	keys.HalfPageUp.SetEnabled(m.focused)
	keys.HalfPageDown.SetEnabled(m.focused)
	keys.FirstRow.SetEnabled(m.focused)
	keys.LastRow.SetEnabled(m.focused)
	keys.CopyCell.SetEnabled(m.focused)
	keys.CopyRow.SetEnabled(m.focused)
	keys.OpenReference.SetEnabled(m.focused)
	keys.BrowseFilter.SetEnabled(m.focused && !m.queryActive)
	keys.ClearBrowseFilter.SetEnabled(m.focused && !m.queryActive && len(m.browseFilters) > 0)
	keys.SQL.SetEnabled(!m.focused)
	keys.Columns.SetEnabled(!m.focused)
	keys.Schema.SetEnabled(!m.focused)
	keys.Filter.SetEnabled(!m.focused)
	keys.ShowDescriptions.SetEnabled(!m.focused)
	keys.ShowValues.SetEnabled(!m.focused)
	keys.Refresh.SetEnabled(!m.focused)
	return keys
}
