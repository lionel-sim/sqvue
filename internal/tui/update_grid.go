package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func (m Model) handleGridKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && unicode.IsDigit(msg.Runes[0]) {
		digit := int(msg.Runes[0] - '0')
		if m.countPrefix > 0 || digit > 0 {
			m.countPrefix = min(maxPageSize*1000, m.countPrefix*10+digit)
		}
		return m, nil
	}
	count := m.consumeGridCount()
	if m.loading && (key.Matches(msg, m.keys.Down) || key.Matches(msg, m.keys.Up) || key.Matches(msg, m.keys.PageDown) || key.Matches(msg, m.keys.PageUp)) {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.BrowseFilter):
		if m.queryActive {
			m.status = "row filters are unavailable for SQL results"
			return m, nil
		}
		return m.openBrowseFilter()
	case key.Matches(msg, m.keys.ClearBrowseFilter):
		if m.queryActive {
			m.status = "row filters are unavailable for SQL results"
			return m, nil
		}
		return m.clearBrowseFilters()
	case key.Matches(msg, m.keys.Down):
		return m.moveGridRows(count)
	case key.Matches(msg, m.keys.Up):
		return m.moveGridRows(-count)
	case key.Matches(msg, m.keys.Right):
		return m.moveGridColumn(count), nil
	case key.Matches(msg, m.keys.Left):
		return m.moveGridColumn(-count), nil
	case key.Matches(msg, m.keys.CopyCell):
		return m.copyToClipboard(m.activeCellValue(), "cell")
	case key.Matches(msg, m.keys.CopyRow):
		return m.copyToClipboard(m.activeRowValue(), "row")
	case key.Matches(msg, m.keys.EditCell):
		if m.queryActive {
			m.status = "editing is unavailable for SQL results"
			return m, nil
		}
		if !m.hasPrimaryKey() {
			m.status = "editing requires a table with a primary key"
			return m, nil
		}
		m.status = "cell editing is not available yet"
		return m, nil
	case key.Matches(msg, m.keys.OpenReference):
		return m.followActiveForeignKey()
	case key.Matches(msg, m.keys.HalfPageDown):
		return m.moveGridRow(max(1, len(m.rows)/2)), nil
	case key.Matches(msg, m.keys.HalfPageUp):
		return m.moveGridRow(-max(1, len(m.rows)/2)), nil
	case key.Matches(msg, m.keys.FirstRow):
		m.rowCursor = 0
	case key.Matches(msg, m.keys.LastRow):
		m.rowCursor = max(0, len(m.rows)-1)
	case key.Matches(msg, m.keys.PageDown):
		return m.changeGridPage(+1)
	case key.Matches(msg, m.keys.PageUp):
		return m.changeGridPage(-1)
	}
	return m, nil
}

func (m Model) copyToClipboard(value, kind string) (Model, tea.Cmd) {
	m.copyStatusID++
	return m, writeClipboardCmd(value, kind, m.copyStatusID)
}

func (m *Model) consumeGridCount() int {
	count := max(1, m.countPrefix)
	m.countPrefix = 0
	return count
}

func (m Model) moveGridRows(delta int) (Model, tea.Cmd) {
	if len(m.rows) == 0 {
		return m, nil
	}
	if m.queryActive {
		if m.queryStreaming {
			target := m.rowCursor + delta
			if target >= 0 && target < len(m.rows) {
				m.rowCursor = target
				return m, nil
			}
			if target < 0 {
				if m.page == 0 {
					m.rowCursor = 0
					return m, nil
				}
				m.page, m.pendingRowMoves = m.page-1, target
				return m.startLoadQueryRows()
			}
			if !m.hasNextPage {
				m.rowCursor = len(m.rows) - 1
				return m, nil
			}
			m.page, m.pendingRowMoves = m.page+1, target-len(m.rows)
			return m.startLoadQueryRows()
		}
		absolute := clamp(m.page*m.pageSize+m.rowCursor+delta, 0, max(0, len(m.queryRows)-1))
		m.page, m.rowCursor = absolute/m.pageSize, absolute%m.pageSize
		m.setQueryPage()
		return m, nil
	}
	target := m.rowCursor + delta
	if target >= 0 && target < len(m.rows) {
		m.rowCursor = target
		return m, nil
	}
	if target < 0 {
		if m.page == 0 {
			m.rowCursor = 0
			return m, nil
		}
		m.page, m.pendingRowMoves = m.page-1, target
		return m.startLoadRows()
	}
	if !m.hasNextPage {
		m.rowCursor = len(m.rows) - 1
		return m, nil
	}
	m.page, m.pendingRowMoves = m.page+1, target-len(m.rows)
	return m.startLoadRows()
}

func (m Model) changeGridPage(delta int) (Model, tea.Cmd) {
	page := m.page
	m, cmd := m.changePage(delta)
	if m.page == page {
		return m, cmd
	}
	if delta > 0 {
		m.rowCursor = 0
	} else {
		m.rowCursor = max(0, m.pageSize-1)
	}
	return m, cmd
}
func (m Model) moveGridRow(delta int) Model {
	m.rowCursor = clamp(m.rowCursor+delta, 0, max(0, len(m.rows)-1))
	return m
}
func (m Model) moveGridColumn(delta int) Model {
	m.cellCursor = clamp(m.cellCursor+delta, 0, max(0, m.displayedColumnCount()-1))
	return m
}

func (m Model) activeCellValue() string {
	_, rows := visibleData(m.columns, m.rows, m.visibleColumns)
	if m.rowCursor < 0 || m.rowCursor >= len(rows) || m.cellCursor < 0 || m.cellCursor >= len(rows[m.rowCursor]) {
		return ""
	}
	return rows[m.rowCursor][m.cellCursor]
}
func (m Model) activeRowValue() string {
	_, rows := visibleData(m.columns, m.rows, m.visibleColumns)
	if m.rowCursor < 0 || m.rowCursor >= len(rows) {
		return ""
	}
	return strings.Join(rows[m.rowCursor], "\t")
}
func (m Model) activeColumn() *db.Column {
	indexes := visibleColumnIndexes(m.columns, m.visibleColumns)
	if m.cellCursor < 0 || m.cellCursor >= len(indexes) {
		return nil
	}
	return &m.columns[indexes[m.cellCursor]]
}

func (m Model) followActiveForeignKey() (Model, tea.Cmd) {
	column := m.activeColumn()
	if column == nil || column.ForeignKey == nil {
		m.status = "active cell is not a foreign key"
		return m, nil
	}
	foreignKey, value := column.ForeignKey, m.activeCellValue()
	if value == "NULL" {
		m.status = "cannot open a NULL reference"
		return m, nil
	}
	schema := foreignKey.Schema
	if schema == "" {
		schema = m.currentSchema()
	}
	if schema != m.currentSchema() {
		m.status = "referenced table is in another schema"
		return m, nil
	}
	if m.filterInput.Value() != "" {
		m.filterInput.SetValue("")
		m.applyFilter()
	}
	for i, table := range m.tables {
		if table.Schema == schema && table.Name == foreignKey.Table {
			m.focused, m.selected = true, i
			m.scroll = keepInView(m.selected, m.scroll, tableListHeight, len(m.tables))
			m.resetBrowseContext()
			m.cellCursor, m.mode = 0, modeValues
			m.browseFilters = []db.RowFilter{{Column: foreignKey.Column, Operator: db.FilterEqual, Value: value}}
			return m.startLoadRows()
		}
	}
	m.status = "referenced table is not available"
	return m, nil
}
