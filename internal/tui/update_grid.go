package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func (m Model) handleGridKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.recordGridCount(msg) {
		return m, nil
	}
	count := m.consumeGridCount()
	if m.gridNavigationBlocked(msg) {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Sort):
		return m.sortActiveColumn()
	case key.Matches(msg, m.keys.BrowseFilter):
		return m.handleGridBrowseFilter(false)
	case key.Matches(msg, m.keys.ClearBrowseFilter):
		return m.handleGridBrowseFilter(true)
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
		return m.handleEditCellKey()
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

func (m *Model) recordGridCount(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 || !unicode.IsDigit(msg.Runes[0]) {
		return false
	}
	digit := int(msg.Runes[0] - '0')
	if m.countPrefix > 0 || digit > 0 {
		m.countPrefix = min(maxPageSize*1000, m.countPrefix*10+digit)
	}
	return true
}

func (m Model) gridNavigationBlocked(msg tea.KeyMsg) bool {
	return m.loading && (key.Matches(msg, m.keys.Down) || key.Matches(msg, m.keys.Up) || key.Matches(msg, m.keys.PageDown) || key.Matches(msg, m.keys.PageUp))
}

func (m Model) handleGridBrowseFilter(clear bool) (Model, tea.Cmd) {
	if m.queryActive {
		m.status = "row filters are unavailable for SQL results"
		return m, nil
	}
	if clear {
		return m.clearBrowseFilters()
	}
	return m.openBrowseFilter()
}

func (m Model) handleEditCellKey() (Model, tea.Cmd) {
	if m.queryActive {
		m.status = "editing is unavailable for SQL results"
		return m, nil
	}
	if !m.hasPrimaryKey() {
		m.status = "editing requires a table with a primary key"
		return m, nil
	}
	return m.beginCellEdit()
}

func (m Model) beginCellEdit() (Model, tea.Cmd) {
	column := m.activeColumn()
	if column == nil || m.rowCursor < 0 || m.rowCursor >= len(m.rows) {
		m.status = "no cell is selected"
		return m, nil
	}
	m.cellEditColumn = column.Name
	m.cellEditOriginal = m.activeCellValue()
	m.cellEditInput.Prompt = "Edit " + sanitizeText(column.Name) + ": "
	m.cellEditInput.SetValue(sanitizeText(m.cellEditOriginal))
	m.cellEditInput.Focus()
	m.activeOverlay = overlayCellEdit
	m.status = "edit cell value"
	return m, nil
}

func (m Model) handleCellEditKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.cellEditInput.Blur()
		m.restoreBrowseStatus()
	case key.Matches(msg, m.keys.Confirm):
		m.activeOverlay = overlayCellEditConfirm
		m.cellEditInput.Blur()
		m.status = "confirm cell update"
	default:
		var cmd tea.Cmd
		m.cellEditInput, cmd = m.cellEditInput.Update(msg)
		m.cellEditInput.SetValue(sanitizeText(m.cellEditInput.Value()))
		return m, cmd
	}
	return m, nil
}

func (m Model) handleCellEditConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayCellEdit
		m.cellEditInput.Focus()
		m.status = "edit cell value"
	case key.Matches(msg, m.keys.Confirm):
		updater, ok := m.client.(db.CellUpdater)
		if !ok {
			m.activeOverlay = overlayNone
			m.status = "cell editing is unavailable for this connection"
			return m, nil
		}
		request, err := m.cellUpdateRequest()
		if err != nil {
			m.activeOverlay = overlayNone
			return m.fail("cell update failed", err)
		}
		m.activeOverlay = overlayNone
		m.closeTableStream()
		m.updateID++
		m.loading = true
		m.status = "updating " + sanitizeText(request.Column) + "..."
		return m, updateCellCmd(updater, request, m.timeout, m.updateID)
	}
	return m, nil
}

func (m Model) cellUpdateRequest() (db.CellUpdateRequest, error) {
	table := m.currentTable()
	if table == nil {
		return db.CellUpdateRequest{}, fmt.Errorf("no table is selected")
	}
	column := m.activeColumn()
	if column == nil || m.rowCursor < 0 || m.rowCursor >= len(m.rows) {
		return db.CellUpdateRequest{}, fmt.Errorf("no cell is selected")
	}
	row := m.rows[m.rowCursor]
	primaryKey := make([]db.PrimaryKeyValue, 0)
	for index, candidate := range m.columns {
		if !candidate.IsPrimary {
			continue
		}
		if index >= len(row) {
			return db.CellUpdateRequest{}, fmt.Errorf("primary-key value for %q is unavailable", candidate.Name)
		}
		primaryKey = append(primaryKey, db.PrimaryKeyValue{Column: candidate.Name, Value: row[index]})
	}
	request := db.CellUpdateRequest{Table: *table, Column: column.Name, Value: m.cellEditRequestValue(), PrimaryKey: primaryKey}
	if err := request.Validate(); err != nil {
		return db.CellUpdateRequest{}, err
	}
	return request, nil
}

func (m Model) cellEditRequestValue() any {
	edited := m.cellEditInput.Value()
	if edited == sanitizeText(m.cellEditOriginal) && m.cellEditOriginal != "NULL" {
		return m.cellEditOriginal
	}
	return cellEditValue(edited)
}

// cellEditValue reserves NULL as an explicit request to clear a database value.
// Other values remain strings so drivers can apply their native type coercion.
func cellEditValue(value string) any {
	if value == "NULL" {
		return nil
	}
	return value
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
		return m.moveQueryGridRows(delta)
	}
	return m.movePagedGridRows(delta, Model.startLoadRows)
}

func (m Model) moveQueryGridRows(delta int) (Model, tea.Cmd) {
	if m.queryStreaming {
		return m.movePagedGridRows(delta, Model.startLoadQueryRows)
	}
	absolute := clamp(m.page*m.pageSize+m.rowCursor+delta, 0, max(0, len(m.queryRows)-1))
	m.page, m.rowCursor = absolute/m.pageSize, absolute%m.pageSize
	m.setQueryPage()
	return m, nil
}

func (m Model) movePagedGridRows(delta int, load func(Model) (Model, tea.Cmd)) (Model, tea.Cmd) {
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
		return load(m)
	}
	if !m.hasNextPage {
		m.rowCursor = len(m.rows) - 1
		return m, nil
	}
	m.page, m.pendingRowMoves = m.page+1, target-len(m.rows)
	return load(m)
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
			m.browseSort = db.SortSpec{}
			m.cellCursor, m.mode = 0, modeValues
			m.browseFilters = []db.RowFilter{{Column: foreignKey.Column, Operator: db.FilterEqual, Value: value}}
			return m.startLoadRows()
		}
	}
	m.status = "referenced table is not available"
	return m, nil
}
