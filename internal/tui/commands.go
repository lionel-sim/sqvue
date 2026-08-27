package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func loadTablesCmd(c db.Driver, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		tables, err := c.ListTables(ctx, "public")
		return tablesLoadedMsg{tables: tables, err: err}
	}
}

func loadRowsCmd(c db.Driver, tbl db.Table, limit, offset int, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cols, rows, err := c.Rows(ctx, tbl, limit, offset)
		return rowsLoadedMsg{columns: cols, rows: rows, err: err}
	}
}

func loadDescriptionsCmd(c db.Driver, tbl db.Table, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		info, err := c.DescribeTable(ctx, tbl.Schema, tbl.Name)
		return descriptionsLoadedMsg{info: info, err: err}
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
		m.loading = true
		m.status = fmt.Sprintf("loading %s page %d...", t.String(), m.page+1)
		offset := m.page * m.pageSize
		return m, loadRowsCmd(m.client, *t, m.pageSize, offset, m.timeout)
	}
	return m, nil
}

func (m Model) startLoadDescriptions() (Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.loading = true
		m.status = fmt.Sprintf("loading %s columns...", t.String())
		return m, loadDescriptionsCmd(m.client, *t, m.timeout)
	}
	return m, nil
}
