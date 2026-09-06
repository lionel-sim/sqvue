package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func (m Model) canDeleteRow() bool {
	if !m.focused || m.queryActive || m.mode != modeValues || !m.hasPrimaryKey() {
		return false
	}
	table := m.currentTable()
	if table == nil || table.Type != "table" || m.rowCursor < 0 || m.rowCursor >= len(m.rows) {
		return false
	}
	_, ok := m.client.(db.RowDeleter)
	return ok
}

func (m Model) beginRowDelete() (Model, tea.Cmd) {
	if m.queryActive {
		m.status = "deleting is unavailable for SQL results"
		return m, nil
	}
	table := m.currentTable()
	if table == nil || table.Type != "table" {
		m.status = "deleting is available only for tables"
		return m, nil
	}
	if !m.hasPrimaryKey() {
		m.status = "deleting requires a table with a primary key"
		return m, nil
	}
	if _, ok := m.client.(db.RowDeleter); !ok {
		m.status = "deleting is unavailable for this connection"
		return m, nil
	}
	request, err := m.rowDeleteRequestForActiveRow()
	if err != nil {
		return m.fail("row delete failed", err)
	}
	m.rowDeleteRequest = request
	m.activeOverlay = overlayRowDeleteConfirm
	m.status = "confirm row deletion"
	return m, nil
}

func (m Model) rowDeleteRequestForActiveRow() (db.RowDeleteRequest, error) {
	table := m.currentTable()
	if table == nil {
		return db.RowDeleteRequest{}, fmt.Errorf("no table is selected")
	}
	if m.rowCursor < 0 || m.rowCursor >= len(m.rows) {
		return db.RowDeleteRequest{}, fmt.Errorf("no row is selected")
	}
	row := m.rows[m.rowCursor]
	request := db.RowDeleteRequest{Table: *table}
	for index, column := range m.columns {
		if !column.IsPrimary {
			continue
		}
		if index >= len(row) {
			return db.RowDeleteRequest{}, fmt.Errorf("primary-key value for %q is unavailable", column.Name)
		}
		request.PrimaryKey = append(request.PrimaryKey, db.PrimaryKeyValue{Column: column.Name, Value: row[index]})
	}
	if err := request.Validate(); err != nil {
		return db.RowDeleteRequest{}, err
	}
	return request, nil
}

func (m Model) handleRowDeleteConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.restoreBrowseStatus()
	case key.Matches(msg, m.keys.Confirm):
		deleter, ok := m.client.(db.RowDeleter)
		if !ok {
			m.activeOverlay = overlayNone
			m.status = "deleting is unavailable for this connection"
			return m, nil
		}
		if err := m.rowDeleteRequest.Validate(); err != nil {
			return m.fail("row delete failed", err)
		}
		m.activeOverlay = overlayNone
		m.closeTableStream()
		m.deleteID++
		m.loading = true
		m.status = "deleting row..."
		return m, deleteRowCmd(deleter, m.rowDeleteRequest, m.timeout, m.deleteID)
	}
	return m, nil
}
