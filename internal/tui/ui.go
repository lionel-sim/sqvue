package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
	keymap "sqvue/internal/tui/components/keys"
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
	width    int

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
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tablesLoadedMsg:
		return m.handleTablesLoaded(msg)
	case rowsLoadedMsg:
		return m.handleRowsLoaded(msg)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width = msg.Width
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		m.status = "reloading tables..."
		return m, loadTablesCmd(m.client, m.timeout)
	case key.Matches(msg, m.keys.Down):
		return m.moveSelection(+1)
	case key.Matches(msg, m.keys.Up):
		return m.moveSelection(-1)
	case key.Matches(msg, m.keys.PageDown):
		return m.changePage(+1)
	case key.Matches(msg, m.keys.PageUp):
		return m.changePage(-1)
	}
	return m, nil
}

func (m Model) moveSelection(delta int) (Model, tea.Cmd) {
	target := m.selected + delta
	if target < 0 || target >= len(m.tables) {
		return m, nil
	}
	m.selected = target
	m.page = 0
	return m.startLoadRows()
}

func (m Model) changePage(delta int) (Model, tea.Cmd) {
	if m.page+delta < 0 {
		return m, nil
	}
	m.page += delta
	return m.startLoadRows()
}

func (m Model) handleTablesLoaded(msg tablesLoadedMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		return m.fail("failed to load tables", msg.err)
	}
	m.tables = msg.tables
	m.status = fmt.Sprintf("found %d tables", len(m.tables))
	if len(m.tables) > 0 {
		m.selected = 0
		m.page = 0
		return m.startLoadRows()
	}
	return m, nil
}

func (m Model) handleRowsLoaded(msg rowsLoadedMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		return m.fail("rows error", msg.err)
	}
	m.columns = msg.columns
	m.rows = msg.rows
	if t := m.currentTable(); t != nil {
		m.status = fmt.Sprintf("%s page %d (%d rows)", t.String(), m.page+1, len(m.rows))
	}
	return m, nil
}

func (m Model) fail(prefix string, err error) (Model, tea.Cmd) {
	m.status = fmt.Sprintf("%s: %v", prefix, err)
	m.lastErr = err
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
