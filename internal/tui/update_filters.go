package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func (m Model) clearBrowseFilters() (Model, tea.Cmd) {
	if len(m.browseFilters) == 0 {
		m.status = "no active row filters"
		return m, nil
	}
	m.browseFilters = nil
	m.resetBrowseContext()
	return m.startLoadRows()
}

func (m *Model) resetBrowseContext() {
	m.page = 0
	m.rowCursor = 0
	m.pendingRowMoves = 0
	m.hasNextPage = false
}

func (m Model) openBrowseFilter() (Model, tea.Cmd) {
	column := m.activeColumn()
	if column == nil {
		m.status = "no active column to filter"
		return m, nil
	}
	m.activeOverlay, m.browseFilterOperator = overlayBrowseFilterOperator, db.FilterEqual
	m.browseFilterColumn, m.browseFilterCursor = column.Name, 0
	m.browseFilterInput.SetValue(m.activeCellValue())
	return m, nil
}

var standardBrowseFilterOperators = []db.FilterOperator{db.FilterEqual, db.FilterContains, db.FilterLike, db.FilterGreater, db.FilterLess, db.FilterIsNull, db.FilterIsNotNull}

func (m Model) browseFilterOperators() []db.FilterOperator {
	operators := append([]db.FilterOperator(nil), standardBrowseFilterOperators...)
	if m.client != nil && m.client.DbType() == db.DbTypePostgres {
		operators = append(operators[:3], append([]db.FilterOperator{db.FilterILike}, operators[3:]...)...)
	}
	return operators
}

func (m Model) handleBrowseFilterOperatorKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
	case key.Matches(msg, m.keys.Down):
		m.browseFilterCursor = min(m.browseFilterCursor+1, len(m.browseFilterOperators())-1)
	case key.Matches(msg, m.keys.Up):
		m.browseFilterCursor = max(0, m.browseFilterCursor-1)
	case key.Matches(msg, m.keys.Confirm):
		m.browseFilterOperator = m.browseFilterOperators()[m.browseFilterCursor]
		if m.browseFilterOperator == db.FilterIsNull || m.browseFilterOperator == db.FilterIsNotNull {
			m.browseFilters = append(m.browseFilters, db.RowFilter{Column: m.browseFilterColumn, Operator: m.browseFilterOperator})
			m.resetBrowseContext()
			m.activeOverlay = overlayNone
			return m.startLoadRows()
		}
		m.updateBrowseFilterPrompt(m.browseFilterColumn)
		m.activeOverlay = overlayBrowseFilter
		m.browseFilterInput.Focus()
	}
	return m, nil
}

func (m Model) handleBrowseFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.activeOverlay = overlayNone
		m.browseFilterInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		m.browseFilters = append(m.browseFilters, db.RowFilter{Column: m.browseFilterColumn, Operator: m.browseFilterOperator, Value: m.browseFilterInput.Value()})
		m.resetBrowseContext()
		m.activeOverlay = overlayNone
		m.browseFilterInput.Blur()
		return m.startLoadRows()
	}
	var cmd tea.Cmd
	m.browseFilterInput, cmd = m.browseFilterInput.Update(msg)
	return m, cmd
}

func (m *Model) updateBrowseFilterPrompt(column string) {
	m.browseFilterInput.Prompt = fmt.Sprintf("Filter %s %s ", column, browseFilterOperatorLabel(m.browseFilterOperator))
}
func browseFilterOperatorLabel(operator db.FilterOperator) string {
	switch operator {
	case db.FilterContains:
		return "contains (case-insensitive)"
	case db.FilterLike:
		return "like"
	case db.FilterILike:
		return "ilike"
	case db.FilterGreater:
		return ">"
	case db.FilterLess:
		return "<"
	case db.FilterIsNull:
		return "is null"
	case db.FilterIsNotNull:
		return "is not null"
	default:
		return "="
	}
}
