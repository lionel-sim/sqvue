package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func loadSchemasCmd(c db.Driver, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		schemas, err := c.ListSchemas(ctx)
		return schemasLoadedMsg{requestID: requestID, schemas: schemas, err: err}
	}
}

func loadTablesCmd(c db.Driver, schema string, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		tables, err := c.ListTables(ctx, schema)
		return tablesLoadedMsg{requestID: requestID, tables: tables, err: err}
	}
}

func loadRowsCmd(c db.Driver, tbl db.Table, limit, offset int, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cols, rows, err := c.Rows(ctx, tbl, limit, offset)
		return rowsLoadedMsg{requestID: requestID, columns: cols, rows: rows, err: err}
	}
}

func loadDescriptionsCmd(c db.Driver, tbl db.Table, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		info, err := c.DescribeTable(ctx, tbl.Schema, tbl.Name)
		return descriptionsLoadedMsg{requestID: requestID, info: info, err: err}
	}
}

func loadCountCmd(c db.Driver, tbl db.Table, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		count, err := c.CountRows(ctx, tbl)
		return countLoadedMsg{requestID: requestID, table: tbl, count: count, err: err}
	}
}

func runQueryCmd(c db.Driver, sql string, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := c.Query(ctx, db.Query{SQL: sql})
		return queryLoadedMsg{requestID: requestID, result: result, err: err}
	}
}

// startLoad dispatches to the loader for the current view mode.
func (m Model) startLoad() (Model, tea.Cmd) {
	if m.mode == modeDescriptions {
		return m.startLoadDescriptions()
	}
	return m.startLoadRows()
}

func (m Model) startLoadRows() (Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.queryActive = false
		m.loading = true
		m.status = fmt.Sprintf("loading %s page %d...", t.String(), m.page+1)
		offset := m.page * m.pageSize
		requestID := m.nextRequestID()
		// Fetch one extra row to determine whether a following page exists.
		return m, loadRowsCmd(m.client, *t, m.pageSize+1, offset, m.timeout, requestID)
	}
	return m, nil
}

func (m Model) startLoadDescriptions() (Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.loading = true
		m.status = fmt.Sprintf("loading %s columns...", t.String())
		requestID := m.nextRequestID()
		return m, loadDescriptionsCmd(m.client, *t, m.timeout, requestID)
	}
	return m, nil
}
