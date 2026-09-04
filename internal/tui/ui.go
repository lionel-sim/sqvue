package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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
	client  db.Driver
	timeout time.Duration

	browserState
	resultState
	gridState
	overlayState
	viewportState
	loadState
	keys keymap.Map
}

// browserState owns schema and table navigation.
type browserState struct {
	schemas      []db.Schema
	schema       int
	schemaCursor int
	schemaScroll int
	allTables    []db.Table
	tables       []db.Table
	selected     int
	scroll       int
}

// resultState owns the currently displayed data and its pagination.
type resultState struct {
	mode             viewMode
	columns          []db.Column
	rows             [][]string
	tableInfo        db.TableInfo
	page             int
	pageSize         int
	hasNextPage      bool
	rowCounts        map[string]int64
	visibleColumns   []bool
	visibleColumnKey string
	referenceFilter  *referenceFilter
	queryState
}

type referenceFilter struct {
	column string
	value  string
}

// gridState tracks whether keyboard input is directed at the displayed rows.
type gridState struct {
	focused    bool
	rowCursor  int
	cellCursor int
}

// queryState retains an ad-hoc query result so it can be paged locally.
type queryState struct {
	queryActive    bool
	queryRows      [][]string
	queryDuration  int64
	queryAffected  int64
	queryTruncated bool
}

// overlayMode identifies the sole interactive overlay that can be active.
type overlayMode uint8

const (
	overlayNone overlayMode = iota
	overlayHelp
	overlaySchemaPicker
	overlayFilter
	overlaySQL
	overlayColumnPicker
	overlayRowDetail
)

// overlayState owns transient inputs, pickers, and the help modal.
type overlayState struct {
	activeOverlay  overlayMode
	filterInput    textinput.Model
	filterPrevious string
	sqlInput       textinput.Model
	help           help.Model
	columnCursor   int
	columnScroll   int
	detailScroll   int
}

// viewportState stores the most recent terminal dimensions.
type viewportState struct {
	width  int
	height int
}

// loadState tracks asynchronous work and its user-facing outcome.
type loadState struct {
	status  string
	loading bool
	lastErr error
	loadID  uint64
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
		client:  opts.Client,
		timeout: opts.Timeout,
		resultState: resultState{
			pageSize:  maxPageSize,
			rowCounts: make(map[string]int64),
		},
		overlayState: overlayState{
			filterInput: filter,
			sqlInput:    sql,
			help:        help.New(),
		},
		loadState: loadState{
			status:  "loading tables...",
			loading: true,
		},
		keys: keymap.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return loadSchemasCmd(m.client, m.timeout, m.loadID)
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
	case clipboardWrittenMsg:
		if msg.err != nil {
			return m.fail("copy failed", msg.err)
		}
		m.status = "copied " + msg.kind
		m.lastErr = nil
		return m, nil
	}
	return m, nil
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	oldSize := m.pageSize
	m.width = msg.Width
	m.height = msg.Height
	m.filterInput.Width = inputWidth(msg.Width, m.filterInput.Prompt)
	m.sqlInput.Width = inputWidth(msg.Width, m.sqlInput.Prompt)
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

func inputWidth(terminalWidth int, prompt string) int {
	return max(1, terminalWidth-ansi.StringWidth(prompt))
}

func (m Model) computedPageSize() int {
	return clamp(m.height-reservedRows, 1, maxPageSize)
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.activeOverlay {
	case overlayHelp:
		m.activeOverlay = overlayNone
		return m, nil
	case overlaySQL:
		return m.handleSQLKey(msg)
	case overlayColumnPicker:
		return m.handleColumnsKey(msg)
	case overlayRowDetail:
		return m.handleRowDetailKey(msg)
	case overlayFilter:
		return m.handleFilterKey(msg)
	case overlaySchemaPicker:
		return m.handleSchemaKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Help):
		m.activeOverlay = overlayHelp
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		if m.focused && msg.String() == "esc" {
			m.focused = false
			return m, nil
		}
		return m, tea.Quit
	case key.Matches(msg, m.keys.Confirm):
		if m.focused {
			m.activeOverlay = overlayRowDetail
			m.detailScroll = 0
			return m, nil
		}
		if m.mode == modeValues && len(m.rows) > 0 {
			m.focused = true
		}
		return m, nil
	case m.focused:
		return m.handleGridKey(msg)
	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		m.status = "reloading tables..."
		requestID := m.nextRequestID()
		return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout, requestID)
	case key.Matches(msg, m.keys.Schema):
		if len(m.schemas) > 0 {
			m.activeOverlay = overlaySchemaPicker
			m.schemaCursor = m.schema
			m.schemaScroll = keepInView(m.schemaCursor, m.schemaScroll, tableListHeight, len(m.schemas))
			m.status = "select a schema"
		}
		return m, nil
	case key.Matches(msg, m.keys.Filter):
		m.filterPrevious = m.filterInput.Value()
		m.activeOverlay = overlayFilter
		m.filterInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.SQL):
		m.activeOverlay = overlaySQL
		m.sqlInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Columns):
		if m.mode == modeValues && len(m.columns) > 0 {
			m.activeOverlay = overlayColumnPicker
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

func (m Model) handleRowDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || key.Matches(msg, m.keys.Confirm):
		m.activeOverlay = overlayNone
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.detailScroll = min(m.detailScroll+1, m.maxDetailScroll())
	case key.Matches(msg, m.keys.Up):
		m.detailScroll = max(0, m.detailScroll-1)
	}
	return m, nil
}

func (m Model) handleGridKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Down):
		if m.rowCursor == len(m.rows)-1 {
			return m.changeGridPage(+1)
		}
		return m.moveGridRow(+1), nil
	case key.Matches(msg, m.keys.Up):
		if m.rowCursor == 0 {
			return m.changeGridPage(-1)
		}
		return m.moveGridRow(-1), nil
	case key.Matches(msg, m.keys.Right):
		return m.moveGridColumn(+1), nil
	case key.Matches(msg, m.keys.Left):
		return m.moveGridColumn(-1), nil
	case key.Matches(msg, m.keys.CopyCell):
		return m, writeClipboardCmd(m.activeCellValue(), "cell")
	case key.Matches(msg, m.keys.CopyRow):
		return m, writeClipboardCmd(m.activeRowValue(), "row")
	case key.Matches(msg, m.keys.OpenReference):
		return m.followActiveForeignKey()
	case key.Matches(msg, m.keys.HalfPageDown):
		return m.moveGridRow(max(1, len(m.rows)/2)), nil
	case key.Matches(msg, m.keys.HalfPageUp):
		return m.moveGridRow(-max(1, len(m.rows)/2)), nil
	case key.Matches(msg, m.keys.FirstRow):
		m.rowCursor = 0
		return m, nil
	case key.Matches(msg, m.keys.LastRow):
		m.rowCursor = max(0, len(m.rows)-1)
		return m, nil
	case key.Matches(msg, m.keys.PageDown):
		return m.changeGridPage(+1)
	case key.Matches(msg, m.keys.PageUp):
		return m.changeGridPage(-1)
	}
	return m, nil
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

	foreignKey := column.ForeignKey
	value := m.activeCellValue()
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
			m.focused = true
			m.selected = i
			m.scroll = keepInView(m.selected, m.scroll, tableListHeight, len(m.tables))
			m.page = 0
			m.rowCursor = 0
			m.cellCursor = 0
			m.mode = modeValues
			m.referenceFilter = &referenceFilter{column: foreignKey.Column, value: value}
			return m.startLoadRows()
		}
	}
	m.status = "referenced table is not available"
	return m, nil
}

func (m Model) handleColumnsKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc" || key.Matches(msg, m.keys.Confirm):
		m.activeOverlay = overlayNone
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
		m.activeOverlay = overlayNone
		m.sqlInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		sql := strings.TrimSpace(m.sqlInput.Value())
		if sql == "" {
			return m, nil
		}
		m.activeOverlay = overlayNone
		m.sqlInput.Blur()
		m.loading = true
		m.status = "running query..."
		requestID := m.nextRequestID()
		return m, runQueryCmd(m.client, sql, m.timeout, requestID)
	}
	var cmd tea.Cmd
	m.sqlInput, cmd = m.sqlInput.Update(msg)
	return m, cmd
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" || key.Matches(msg, m.keys.Confirm) {
		if msg.String() == "esc" {
			m.filterInput.SetValue(m.filterPrevious)
		}
		m.activeOverlay = overlayNone
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
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.status = fmt.Sprintf("schema %s", m.currentSchema())
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		if m.schemaCursor < len(m.schemas)-1 {
			m.schemaCursor++
		}
		m.schemaScroll = keepInView(m.schemaCursor, m.schemaScroll, tableListHeight, len(m.schemas))
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.schemaCursor > 0 {
			m.schemaCursor--
		}
		m.schemaScroll = keepInView(m.schemaCursor, m.schemaScroll, tableListHeight, len(m.schemas))
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.schema = m.schemaCursor
		m.activeOverlay = overlayNone
		m.selected, m.scroll, m.page = 0, 0, 0
		m.filterInput.SetValue("")
		m.applyFilter()
		m.loading = true
		m.status = fmt.Sprintf("loading %s tables...", m.currentSchema())
		requestID := m.nextRequestID()
		return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout, requestID)
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
	m.rowCursor = 0
	m.cellCursor = 0
	m.referenceFilter = nil
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
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
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
		m.rowCursor = 0
		m.cellCursor = 0
		m.mode = modeValues
		return m.startLoadRows()
	}
	return m, nil
}

func (m Model) handleSchemasLoaded(msg schemasLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
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
	requestID := m.nextRequestID()
	return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout, requestID)
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
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		return m.fail("rows error", msg.err)
	}
	m.columns = msg.columns
	if t := m.currentTable(); t != nil {
		m.ensureVisibleColumns(t.String())
	}
	m.rows = msg.rows
	m.hasNextPage = len(m.rows) > m.pageSize
	if m.hasNextPage {
		m.rows = m.rows[:m.pageSize]
	}
	if m.rowCursor >= len(m.rows) {
		m.rowCursor = max(0, len(m.rows)-1)
	}
	m.cellCursor = clamp(m.cellCursor, 0, max(0, m.displayedColumnCount()-1))
	m.lastErr = nil
	if t := m.currentTable(); t != nil {
		m.setRowsStatus(*t)
		if m.referenceFilter == nil {
			if _, ok := m.rowCounts[t.String()]; !ok {
				return m, loadCountCmd(m.client, *t, m.timeout, m.loadID)
			}
		}
	}
	return m, nil
}

func (m Model) handleCountLoaded(msg countLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
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
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		return m.fail("query failed", msg.err)
	}
	m.queryActive = true
	m.referenceFilter = nil
	m.lastErr = nil
	m.mode = modeValues
	m.page = 0
	m.rowCursor = 0
	m.cellCursor = 0
	m.queryDuration = msg.result.DurationMs
	m.queryAffected = msg.result.RowsAffected
	m.queryTruncated = msg.result.Truncated
	m.columns = make([]db.Column, len(msg.result.Columns))
	for i, name := range msg.result.Columns {
		m.columns[i] = db.Column{Name: name}
	}
	m.visibleColumns = make([]bool, len(m.columns))
	m.visibleColumnKey = "query"
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
	if m.queryTruncated {
		m.status += " [limited to 1000 rows]"
	}
}

func (m *Model) setRowsStatus(t db.Table) {
	if m.referenceFilter != nil {
		m.status = fmt.Sprintf("%s where %s = %s (%d rows)", t.String(), m.referenceFilter.column, m.referenceFilter.value, len(m.rows))
		return
	}
	status := fmt.Sprintf("%s page %d (%d rows", t.String(), m.page+1, len(m.rows))
	if count, ok := m.rowCounts[t.String()]; ok {
		status += fmt.Sprintf(" of %d", count)
	}
	m.status = status + ")"
}

func (m *Model) ensureVisibleColumns(key string) {
	if m.visibleColumnKey == key && len(m.visibleColumns) == len(m.columns) {
		return
	}
	m.visibleColumnKey = key
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

func (m Model) displayedColumnCount() int {
	if len(m.visibleColumns) == 0 {
		return len(m.columns)
	}
	return m.visibleColumnCount()
}

func (m Model) columnPickerHeight() int {
	if m.height <= 0 {
		return tableListHeight
	}
	return max(1, m.height-reservedRows-1)
}

func (m Model) handleDescriptionsLoaded(msg descriptionsLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
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

func (m *Model) nextRequestID() uint64 {
	m.loadID++
	return m.loadID
}

func (m Model) isCurrent(requestID uint64) bool {
	return requestID == m.loadID
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
