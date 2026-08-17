package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
	keymap "sqvue/internal/tui/components/keys"
)

type (
	tablesLoadedMsg struct {
		tables []db.Table
		err    error
	}
	rowsLoadedMsg struct {
		columns []db.Column
		rows    [][]string
		err     error
	}
)

type Options struct {
	Client   db.Driver
	PageSize int
	Timeout  time.Duration
}

type Model struct {
	client   db.Driver
	timeout  time.Duration
	tables   []db.Table
	selected int

	columns []db.Column
	rows    [][]string

	status string

	page     int
	pageSize int
	loading  bool
	lastErr  error

	keys keymap.Map
}

func New(opts Options) Model {
	return Model{
		client:   opts.Client,
		timeout:  opts.Timeout,
		status:   "loading tables...",
		pageSize: opts.PageSize,
		loading:  true,
		keys:     keymap.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return loadTablesCmd(m.client, m.timeout)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			m.status = "reloading tables..."
			return m, loadTablesCmd(m.client, m.timeout)

		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.tables)-1 {
				m.selected++
				m.page = 0
				return m.startLoadRows()
			}

		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
				m.page = 0
				return m.startLoadRows()
			}

		case key.Matches(msg, m.keys.PageDown):
			m.page++
			return m.startLoadRows()

		case key.Matches(msg, m.keys.PageUp):
			if m.page > 0 {
				m.page--
				return m.startLoadRows()
			}
		}

	case tablesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("failed to load tables: %v", msg.err)
			m.lastErr = msg.err
			return m, nil
		}
		m.tables = msg.tables
		m.status = fmt.Sprintf("found %d tables", len(m.tables))
		if len(m.tables) > 0 {
			m.selected = 0
			m.page = 0
			return m.startLoadRows()
		}

	case rowsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("rows error: %v", msg.err)
			m.lastErr = msg.err
			return m, nil
		}
		m.columns = msg.columns
		m.rows = msg.rows
		if t := m.currentTable(); t != nil {
			m.status = fmt.Sprintf("%s page %d (%d rows)", t.String(), m.page+1, len(m.rows))
		}
	}

	return m, nil
}

func (m Model) View() string {
	return Render(m)
}

func (m *Model) currentTable() *db.Table {
	if m.selected < 0 || m.selected >= len(m.tables) {
		return nil
	}
	return &m.tables[m.selected]
}

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

func (m Model) startLoadRows() (tea.Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.loading = true
		m.status = fmt.Sprintf("loading %s page %d...", t.String(), m.page+1)
		offset := m.page * m.pageSize
		return m, loadRowsCmd(m.client, *t, m.pageSize, offset, m.timeout)
	}
	return m, nil
}

func Render(m Model) string {
	var b strings.Builder

	b.WriteString("Tables (j/k to navigate, pgup/pgdown to page, q to quit)\n")
	if len(m.tables) == 0 {
		b.WriteString("  (no tables found)\n")
	} else {
		for i, t := range m.tables {
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			b.WriteString(prefix + t.String() + "\n")
		}
	}

	b.WriteString("\n")
	if m.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if len(m.columns) > 0 {
		for i, col := range m.columns {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(col.Name)
		}

		for _, row := range m.rows {
			b.WriteString(strings.Join(row, " | ") + "\n")
		}
	} else {
		b.WriteString("(no rows to display)\n")
	}

	b.WriteString("\n")
	b.WriteString("Status: " + m.status)

	return b.String()
}
