package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
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

	page           int
	pageSize       int
	hasNextPage    bool
	loading        bool
	lastErr        error
	width          int
	height         int
	filterInput    textinput.Model
	filtering      bool
	sqlInput       textinput.Model
	sqlMode        bool
	queryActive    bool
	queryRows      [][]string
	queryDuration  int64
	queryAffected  int64
	rowCounts      map[string]int64
	visibleColumns []bool
	showColumns    bool
	columnCursor   int
	columnScroll   int
	help           help.Model
	showHelp       bool

	keys keymap.Map
}

func New(opts Options) Model {
	filter := textinput.New()
	filter.Prompt = "Filter tables: "
	filter.Placeholder = "type to search"
	sql := textinput.New()
	sql.Prompt = "SQL> "
	sql.Placeholder = "SELECT * FROM ..."
	sql.CharLimit = 0
	return Model{
		client:      opts.Client,
		timeout:     opts.Timeout,
		status:      "loading tables...",
		pageSize:    maxPageSize,
		loading:     true,
		keys:        keymap.Default(),
		filterInput: filter,
		sqlInput:    sql,
		rowCounts:   make(map[string]int64),
		help:        help.New(),
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
	case countLoadedMsg:
		return m.handleCountLoaded(msg)
	case queryLoadedMsg:
		return m.handleQueryLoaded(msg)
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
	if m.pageSize != oldSize && m.queryActive {
		m.setQueryPage()
		return m, nil
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
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	if m.sqlMode {
		return m.handleSQLKey(msg)
	}
	if m.showColumns {
		return m.handleColumnsKey(msg)
	}
	if m.filtering {
		return m.handleFilterKey(msg)
	}
	if m.showSchemas {
		return m.handleSchemaKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
		return m, nil
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
	case key.Matches(msg, m.keys.SQL):
		m.sqlMode = true
		m.sqlInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Columns):
		if m.mode == modeValues && len(m.columns) > 0 {
			m.showColumns = true
			m.columnCursor = 0
			m.columnScroll = 0
			m.status = "choose visible columns"
		}
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

func (m Model) handleColumnsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || key.Matches(msg, m.keys.Confirm):
		m.showColumns = false
		if m.queryActive {
			m.setQueryPage()
		} else if t := m.currentTable(); t != nil {
			m.setRowsStatus(*t)
		}
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		if m.columnCursor < len(m.columns)-1 {
			m.columnCursor++
		}
	case key.Matches(msg, m.keys.Up):
		if m.columnCursor > 0 {
			m.columnCursor--
		}
	case key.Matches(msg, m.keys.Toggle):
		if m.visibleColumnCount() == 1 && m.visibleColumns[m.columnCursor] {
			m.status = "at least one column must remain visible"
			return m, nil
		}
		m.visibleColumns[m.columnCursor] = !m.visibleColumns[m.columnCursor]
	}
	m.columnScroll = keepInView(m.columnCursor, m.columnScroll, m.columnPickerHeight(), len(m.columns))
	return m, nil
}

func (m Model) handleSQLKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.sqlMode = false
		m.sqlInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		sql := strings.TrimSpace(m.sqlInput.Value())
		if sql == "" {
			return m, nil
		}
		m.sqlMode = false
		m.sqlInput.Blur()
		m.loading = true
		m.status = "running query..."
		return m, runQueryCmd(m.client, sql, m.timeout)
	}
	var cmd tea.Cmd
	m.sqlInput, cmd = m.sqlInput.Update(msg)
	return m, cmd
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" || key.Matches(msg, m.keys.Confirm) {
		m.filtering = false
		m.filterInput.Blur()
		m.applyFilter()
		return m.startLoad()
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
		m.queryActive = false
		m.mode = modeDescriptions
		return m.startLoadDescriptions()
	}
	return m, nil
}

func (m Model) showValues() (Model, tea.Cmd) {
	if m.mode != modeValues || m.queryActive {
		m.queryActive = false
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
	if m.queryActive {
		if delta > 0 && (m.page+1)*m.pageSize >= len(m.queryRows) {
			return m, nil
		}
		m.page += delta
		m.setQueryPage()
		return m, nil
	}
	if delta > 0 && !m.hasNextPage {
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
	m.lastErr = nil
	m.rowCounts = make(map[string]int64)
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
	m.lastErr = nil
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
	m.ensureVisibleColumns()
	m.rows = msg.rows
	m.hasNextPage = len(m.rows) > m.pageSize
	if m.hasNextPage {
		m.rows = m.rows[:m.pageSize]
	}
	m.lastErr = nil
	if t := m.currentTable(); t != nil {
		m.setRowsStatus(*t)
		if _, ok := m.rowCounts[t.String()]; !ok {
			return m, loadCountCmd(m.client, *t, m.timeout)
		}
	}
	return m, nil
}

func (m Model) handleCountLoaded(msg countLoadedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	m.rowCounts[msg.table.String()] = msg.count
	if t := m.currentTable(); t != nil && *t == msg.table && !m.queryActive {
		m.setRowsStatus(*t)
	}
	return m, nil
}

func (m Model) handleQueryLoaded(msg queryLoadedMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		return m.fail("query failed", msg.err)
	}
	m.queryActive = true
	m.lastErr = nil
	m.mode = modeValues
	m.page = 0
	m.queryDuration = msg.result.DurationMs
	m.queryAffected = msg.result.RowsAffected
	m.columns = make([]db.Column, len(msg.result.Columns))
	for i, name := range msg.result.Columns {
		m.columns[i] = db.Column{Name: name}
	}
	m.visibleColumns = make([]bool, len(m.columns))
	for i := range m.visibleColumns {
		m.visibleColumns[i] = true
	}
	m.queryRows = make([][]string, len(msg.result.Rows))
	for i, row := range msg.result.Rows {
		m.queryRows[i] = make([]string, len(row))
		for j, value := range row {
			m.queryRows[i][j] = formatQueryValue(value)
		}
	}
	m.setQueryPage()
	m.sqlInput.SetValue("")
	return m, nil
}

func formatQueryValue(value any) string {
	if value == nil {
		return "NULL"
	}
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return fmt.Sprint(value)
}

func (m *Model) setQueryPage() {
	start := m.page * m.pageSize
	if start > len(m.queryRows) {
		start = len(m.queryRows)
	}
	end := min(start+m.pageSize, len(m.queryRows))
	m.rows = m.queryRows[start:end]
	m.status = fmt.Sprintf("query page %d (%d/%d rows, %d ms, %d affected)", m.page+1, len(m.rows), len(m.queryRows), m.queryDuration, m.queryAffected)
}

func (m *Model) setRowsStatus(t db.Table) {
	status := fmt.Sprintf("%s page %d (%d rows", t.String(), m.page+1, len(m.rows))
	if count, ok := m.rowCounts[t.String()]; ok {
		status += fmt.Sprintf(" of %d", count)
	}
	m.status = status + ")"
}

func (m *Model) ensureVisibleColumns() {
	if len(m.visibleColumns) == len(m.columns) {
		return
	}
	m.visibleColumns = make([]bool, len(m.columns))
	for i := range m.visibleColumns {
		m.visibleColumns[i] = true
	}
}

func (m Model) visibleColumnCount() int {
	count := 0
	for _, visible := range m.visibleColumns {
		if visible {
			count++
		}
	}
	return count
}

func (m Model) columnPickerHeight() int {
	if m.height <= 0 {
		return tableListHeight
	}
	return max(1, m.height-reservedRows-1)
}

func (m Model) handleDescriptionsLoaded(msg descriptionsLoadedMsg) (Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		return m.fail("describe failed", msg.err)
	}
	m.tableInfo = msg.info
	m.lastErr = nil
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
