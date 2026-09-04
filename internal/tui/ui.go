package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
	keymap "sqvue/internal/tui/components/keys"
)

type Options struct {
	Client  db.Driver
	Timeout time.Duration
}

type viewMode int

const (
	modeValues viewMode = iota
	modeDescriptions
)

type Model struct {
	client      db.Driver
	timeout     time.Duration
	schemas     []db.Schema
	schema      int
	showSchemas bool
	allTables   []db.Table
	tables      []db.Table
	selected    int
	scroll      int
	mode        viewMode

	columns []db.Column
	rows    [][]string

	tableInfo db.TableInfo

	status string

	page        int
	pageSize    int
	loading     bool
	lastErr     error
	width       int
	height      int
	filterInput textinput.Model
	filtering   bool

	keys keymap.Map
}

func New(opts Options) Model {
	filter := textinput.New()
	filter.Prompt = "Filter tables: "
	filter.Placeholder = "type to search"
	return Model{
		client:      opts.Client,
		timeout:     opts.Timeout,
		status:      "loading tables...",
		pageSize:    maxPageSize,
		loading:     true,
		keys:        keymap.Default(),
		filterInput: filter,
	}
}

func (m Model) Init() tea.Cmd {
	return loadSchemasCmd(m.client, m.timeout)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case schemasLoadedMsg:
		return m.handleSchemasLoaded(msg)
	case tablesLoadedMsg:
		return m.handleTablesLoaded(msg)
	case rowsLoadedMsg:
		return m.handleRowsLoaded(msg)
	case descriptionsLoadedMsg:
		return m.handleDescriptionsLoaded(msg)
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	oldSize := m.pageSize
	m.width = msg.Width
	m.height = msg.Height
	if msg.Height > 0 {
		m.pageSize = m.computedPageSize()
	}
	if m.pageSize != oldSize && len(m.tables) > 0 {
		return m.startLoad()
	}
	return m, nil
}

func (m Model) computedPageSize() int {
	return clamp(m.height-reservedRows, 1, maxPageSize)
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.filtering {
		return m.handleFilterKey(msg)
	}
	if m.showSchemas {
		return m.handleSchemaKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		m.status = "reloading tables..."
		return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout)
	case key.Matches(msg, m.keys.Schema):
		if len(m.schemas) > 0 {
			m.showSchemas = true
			m.status = "select a schema"
		}
		return m, nil
	case key.Matches(msg, m.keys.Filter):
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Down):
		return m.moveSelection(+1)
	case key.Matches(msg, m.keys.Up):
		return m.moveSelection(-1)
	case key.Matches(msg, m.keys.PageDown):
		return m.changePage(+1)
	case key.Matches(msg, m.keys.PageUp):
		return m.changePage(-1)
	case key.Matches(msg, m.keys.ShowDescriptions):
		return m.showDescriptions()
	case key.Matches(msg, m.keys.ShowValues):
		return m.showValues()
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.filtering = false
		m.filterInput.Blur()
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m Model) handleSchemaKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.showSchemas = false
		m.status = fmt.Sprintf("schema %s", m.currentSchema())
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.schema < len(m.schemas)-1 {
			m.schema++
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.schema > 0 {
			m.schema--
		}
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.showSchemas = false
		m.selected, m.scroll, m.page = 0, 0, 0
		m.filterInput.SetValue("")
		m.applyFilter()
		m.loading = true
		m.status = fmt.Sprintf("loading %s tables...", m.currentSchema())
		return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout)
	}
	return m, nil
}

func (m Model) showDescriptions() (Model, tea.Cmd) {
	if m.mode != modeDescriptions {
		m.mode = modeDescriptions
		return m.startLoadDescriptions()
	}
	return m, nil
}

func (m Model) showValues() (Model, tea.Cmd) {
	if m.mode != modeValues {
		m.mode = modeValues
		m.page = 0
		return m.startLoadRows()
	}
	return m, nil
}

func (m Model) moveSelection(delta int) (Model, tea.Cmd) {
	target := m.selected + delta
	if target < 0 || target >= len(m.tables) {
		return m, nil
	}
	m.selected = target
	m.scroll = keepInView(m.selected, m.scroll, tableListHeight, len(m.tables))
	m.page = 0
	return m.startLoad()
}

// keepInView returns the scroll offset that keeps selected within a window of
// the given height, scrolling the list as the selection moves past its edges.
func keepInView(selected, offset, height, count int) int {
	if count <= height {
		return 0
	}
	if selected < offset {
		return selected
	}
	if selected >= offset+height {
		return selected - height + 1
	}
	return offset
}

func (m Model) changePage(delta int) (Model, tea.Cmd) {
	if m.mode != modeValues {
		return m, nil
	}
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
	m.allTables = msg.tables
	m.applyFilter()
	m.status = fmt.Sprintf("found %d tables", len(m.tables))
	if len(m.tables) > 0 {
		m.selected = 0
		m.scroll = 0
		m.page = 0
		m.mode = modeValues
		return m.startLoadRows()
	}
	return m, nil
}

func (m Model) handleSchemasLoaded(msg schemasLoadedMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		return m.fail("failed to load schemas", msg.err)
	}
	m.schemas = msg.schemas
	if len(m.schemas) == 0 {
		return m.fail("failed to load schemas", fmt.Errorf("no user schemas found"))
	}
	for i, schema := range m.schemas {
		if schema.Name == "public" {
			m.schema = i
			break
		}
	}
	m.loading = true
	m.status = fmt.Sprintf("loading %s tables...", m.currentSchema())
	return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout)
}

func (m *Model) applyFilter() {
	needle := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.tables = m.tables[:0]
	for _, table := range m.allTables {
		if needle == "" || strings.Contains(strings.ToLower(table.Name), needle) {
			m.tables = append(m.tables, table)
		}
	}
	m.selected = 0
	m.scroll = 0
	m.page = 0
}

func (m Model) currentSchema() string {
	if m.schema < 0 || m.schema >= len(m.schemas) {
		return ""
	}
	return m.schemas[m.schema].Name
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

func (m Model) handleDescriptionsLoaded(msg descriptionsLoadedMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		return m.fail("describe failed", msg.err)
	}
	m.tableInfo = msg.info
	if t := m.currentTable(); t != nil {
		m.status = fmt.Sprintf("%s (%d columns)", t.String(), len(msg.info.Columns))
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
