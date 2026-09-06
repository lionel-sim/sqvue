package tui

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

// toggleSort cycles the active column through ascending, descending, and the
// driver's stable default ordering.
func toggleSort(current db.SortSpec, column string) db.SortSpec {
	if current.Column != column {
		return db.SortSpec{Column: column}
	}
	if !current.Descending {
		return db.SortSpec{Column: column, Descending: true}
	}
	return db.SortSpec{}
}

func formatSort(sort db.SortSpec) string {
	if sort.Column == "" {
		return ""
	}
	direction := "asc"
	if sort.Descending {
		direction = "desc"
	}
	return sanitizeText(sort.Column) + " " + direction
}

func (m Model) sortActiveColumn() (Model, tea.Cmd) {
	column := m.activeColumn()
	if column == nil {
		m.status = "no column is selected for sorting"
		return m, nil
	}
	if m.queryActive {
		return m.sortQuery(column.Name)
	}
	m.browseSort = toggleSort(m.browseSort, column.Name)
	m.page, m.rowCursor, m.pendingRowMoves = 0, 0, 0
	m.closeTableStream()
	return m.startLoadRows()
}

func (m Model) sortQuery(column string) (Model, tea.Cmd) {
	nextSort := toggleSort(m.querySort, column)
	if !m.queryStreaming {
		m.querySort = nextSort
		m.page, m.rowCursor, m.pendingRowMoves = 0, 0, 0
		m.queryRows = append([][]string(nil), m.queryBaseRows...)
		sortQueryRows(m.queryRows, m.columns, m.querySort)
		m.setQueryPage()
		return m, nil
	}
	sorter, ok := m.client.(db.QuerySorter)
	if !ok {
		m.status = "sorting streamed query results is unavailable for this connection"
		return m, nil
	}
	query, err := sorter.SortedQuery(db.Query{SQL: m.querySourceSQL}, nextSort)
	if err != nil {
		return m.fail("sort query failed", err)
	}
	m.querySort = nextSort
	m.page, m.rowCursor, m.pendingRowMoves = 0, 0, 0
	m.querySQL = query.SQL
	m.closeQueryStream()
	return m.startLoadQueryRows()
}

func sortQueryRows(rows [][]string, columns []db.Column, spec db.SortSpec) {
	if spec.Column == "" {
		return
	}
	index := -1
	for i, column := range columns {
		if column.Name == spec.Column {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := "", ""
		if index < len(rows[i]) {
			left = rows[i][index]
		}
		if index < len(rows[j]) {
			right = rows[j][index]
		}
		if spec.Descending {
			return left > right
		}
		return left < right
	})
}
