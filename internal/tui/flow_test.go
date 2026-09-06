package tui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

type fakeDriver struct {
	dbType         db.DbType
	schemas        []db.Schema
	tables         []db.Table
	cols           []db.Column
	rows           [][]string
	describeCols   []db.Column
	lastLimit      int
	lastOffset     int
	lastBrowse     db.BrowseRequest
	lastSchema     string
	lastDescribe   string
	describeCalls  int
	count          int64
	queryResult    db.Result
	lastQuery      string
	backupCaps     db.BackupCapabilities
	lastBackup     db.BackupRequest
	backupErr      error
	lastCellUpdate db.CellUpdateRequest
	cellUpdateErr  error
}

type streamFakeDriver struct {
	fakeDriver
	streamRequests []db.TableRowStreamRequest
	streams        []*fakeRowStream
}

type fakeRowStream struct {
	rows   [][]string
	index  int
	closed bool
}

type queryStreamFakeDriver struct {
	fakeDriver
	queryRequests []db.Query
	queryStreams  []*fakeQueryRowStream
}

type fakeQueryRowStream struct {
	fakeRowStream
	columns  []string
	affected int64
}

func (s *fakeQueryRowStream) Columns() []string   { return append([]string(nil), s.columns...) }
func (s *fakeQueryRowStream) RowsAffected() int64 { return s.affected }
func (*fakeQueryRowStream) DurationMs() int64     { return 1 }

func (f *queryStreamFakeDriver) OpenQueryRowStream(_ context.Context, query db.Query) (db.QueryRowStream, error) {
	stream := &fakeQueryRowStream{fakeRowStream: fakeRowStream{rows: f.rows}, columns: []string{"id"}}
	f.queryRequests = append(f.queryRequests, query)
	f.queryStreams = append(f.queryStreams, stream)
	return stream, nil
}

func (*queryStreamFakeDriver) SortedQuery(query db.Query, sort db.SortSpec) (db.Query, error) {
	if sort.Column == "" {
		return query, nil
	}
	direction := "asc"
	if sort.Descending {
		direction = "desc"
	}
	query.SQL += " order by " + sort.Column + " " + direction
	return query, nil
}

func (s *fakeRowStream) Next() ([]string, error) {
	if s.index >= len(s.rows) {
		return nil, io.EOF
	}
	row := s.rows[s.index]
	s.index++
	return row, nil
}

func (s *fakeRowStream) Close() error {
	s.closed = true
	return nil
}

func (f *streamFakeDriver) OpenTableRowStream(_ context.Context, request db.TableRowStreamRequest) ([]db.Column, db.RowStream, error) {
	stream := &fakeRowStream{rows: f.rows}
	f.streamRequests = append(f.streamRequests, request)
	f.streams = append(f.streams, stream)
	return f.cols, stream, nil
}

func (f *fakeDriver) DbType() db.DbType {
	if f.dbType != "" {
		return f.dbType
	}
	return db.DbTypePostgres
}
func (f *fakeDriver) Connect(context.Context, db.ConnectConfig) error { return nil }
func (f *fakeDriver) Close() error                                    { return nil }
func (f *fakeDriver) Ping(context.Context) error                      { return nil }
func (f *fakeDriver) ListSchemas(context.Context) ([]db.Schema, error) {
	return f.schemas, nil
}
func (f *fakeDriver) ListTables(_ context.Context, schema string) ([]db.Table, error) {
	f.lastSchema = schema
	return f.tables, nil
}
func (f *fakeDriver) DescribeTable(_ context.Context, schema, table string) (db.TableInfo, error) {
	f.lastDescribe = table
	f.describeCalls++
	return db.TableInfo{Schema: schema, Name: table, Columns: f.describeCols}, nil
}
func (f *fakeDriver) Query(_ context.Context, q db.Query) (db.Result, error) {
	f.lastQuery = q.SQL
	return f.queryResult, nil
}
func (f *fakeDriver) Rows(ctx context.Context, tbl db.Table, limit, offset int) ([]db.Column, [][]string, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	return f.cols, f.pageRows(limit, offset), nil
}
func (f *fakeDriver) BrowseRows(_ context.Context, req db.BrowseRequest) ([]db.Column, [][]string, error) {
	f.lastBrowse = req
	return f.cols, f.pageRows(req.Limit, req.Offset), nil
}

func (f *fakeDriver) pageRows(limit, offset int) [][]string {
	if offset >= len(f.rows) {
		return nil
	}
	end := min(offset+limit, len(f.rows))
	return f.rows[offset:end]
}
func (f *fakeDriver) CountBrowseRows(context.Context, db.BrowseRequest) (int64, error) {
	return f.count, nil
}
func (f *fakeDriver) CountRows(context.Context, db.Table) (int64, error) { return f.count, nil }
func (f *fakeDriver) BackupCapabilities() db.BackupCapabilities {
	if f.backupCaps != (db.BackupCapabilities{}) {
		return f.backupCaps
	}
	return db.BackupCapabilities{Database: true, FileExtension: "sql"}
}
func (f *fakeDriver) Backup(_ context.Context, request db.BackupRequest) error {
	f.lastBackup = request
	return f.backupErr
}
func (f *fakeDriver) UpdateCell(_ context.Context, request db.CellUpdateRequest) error {
	f.lastCellUpdate = request
	return f.cellUpdateErr
}

func makeRows(n int) [][]string {
	rows := make([][]string, n)
	for i := range rows {
		rows[i] = []string{"x"}
	}
	return rows
}

func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestTableBrowsingStreamsForwardAndReopensForPreviousPage(t *testing.T) {
	client := &streamFakeDriver{fakeDriver: fakeDriver{
		cols: []db.Column{{Name: "id"}},
		rows: [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}},
	}}
	m := New(Options{Client: client, Timeout: time.Second})
	m.tables = []db.Table{{Schema: "public", Name: "items"}}
	m.pageSize = 2

	m, cmd := m.startLoadRows()
	m, _ = update(m, runCmd(cmd))
	if len(client.streamRequests) != 1 || client.lastLimit != 0 || client.lastBrowse.Limit != 0 {
		t.Fatalf("first page did not use a stream: requests %#v, limit %d, browse %#v", client.streamRequests, client.lastLimit, client.lastBrowse)
	}
	if got := m.rows; len(got) != 2 || got[0][0] != "1" || got[1][0] != "2" || !m.hasNextPage {
		t.Fatalf("first streamed page = %#v, hasNext %t", got, m.hasNextPage)
	}
	if _, ok := m.rowCounts["public.items"]; !ok {
		t.Fatal("first streamed page did not load the table row count")
	}

	m, cmd = m.changePage(+1)
	m, _ = update(m, runCmd(cmd))
	if len(client.streamRequests) != 1 {
		t.Fatalf("forward page reopened stream %d times", len(client.streamRequests))
	}
	if got := m.rows; m.page != 1 || len(got) != 2 || got[0][0] != "3" || got[1][0] != "4" {
		t.Fatalf("second streamed page = page %d, rows %#v", m.page, got)
	}

	m, cmd = m.changePage(-1)
	m, _ = update(m, runCmd(cmd))
	if len(client.streamRequests) != 2 || !client.streams[0].closed {
		t.Fatalf("previous page did not reset its stream: requests %d, closed %t", len(client.streamRequests), client.streams[0].closed)
	}
	if got := m.rows; m.page != 0 || len(got) != 2 || got[0][0] != "1" || got[1][0] != "2" {
		t.Fatalf("reopened first page = page %d, rows %#v", m.page, got)
	}
}

func TestStreamedExactPageBoundaryHasNoPhantomNextPage(t *testing.T) {
	client := &streamFakeDriver{fakeDriver: fakeDriver{
		cols: []db.Column{{Name: "id"}},
		rows: [][]string{{"1"}, {"2"}, {"3"}, {"4"}},
	}}
	m := New(Options{Client: client, Timeout: time.Second})
	m.tables = []db.Table{{Schema: "public", Name: "items"}}
	m.pageSize = 2
	m, cmd := m.startLoadRows()
	m, _ = update(m, runCmd(cmd))
	m, cmd = m.changePage(+1)
	m, _ = update(m, runCmd(cmd))
	if m.hasNextPage {
		t.Fatal("last exact-size page reported a next page")
	}
	page := m.page
	m, cmd = m.changePage(+1)
	if cmd != nil || m.page != page {
		t.Fatalf("phantom next page changed state: command %t, page %d", cmd != nil, m.page)
	}
}

func TestTableStreamResetsForBrowseContextAndResize(t *testing.T) {
	client := &streamFakeDriver{fakeDriver: fakeDriver{
		cols: []db.Column{{Name: "id"}},
		rows: [][]string{{"1"}, {"2"}, {"3"}},
	}}
	m := New(Options{Client: client, Timeout: time.Second})
	m.tables = []db.Table{{Schema: "public", Name: "items"}}
	m.pageSize = 2
	m, cmd := m.startLoadRows()
	m, _ = update(m, runCmd(cmd))
	first := client.streams[0]

	m.browseFilters = []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "2"}}
	m.resetBrowseContext()
	if !first.closed {
		t.Fatal("changing filters did not close the active stream")
	}
	m, cmd = m.startLoadRows()
	m, _ = update(m, runCmd(cmd))
	if len(client.streamRequests) != 2 || len(client.streamRequests[1].Filters) != 1 {
		t.Fatalf("filtered stream request = %#v", client.streamRequests)
	}

	second := client.streams[1]
	m, _ = m.showDescriptions()
	if !second.closed {
		t.Fatal("switching to descriptions did not close the active stream")
	}
	m.mode = modeValues
	m, cmd = m.startLoadRows()
	m, _ = update(m, runCmd(cmd))
	third := client.streams[2]
	m, cmd = update(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	if cmd == nil || !third.closed {
		t.Fatalf("resizing did not reset the active stream: command %v, closed %t", cmd != nil, third.closed)
	}
}

func TestQueryResultsStreamForwardAndReopenForPreviousPage(t *testing.T) {
	client := &queryStreamFakeDriver{fakeDriver: fakeDriver{rows: [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}}}}
	m := New(Options{Client: client, Timeout: time.Second})
	m.pageSize = 2
	m.sqlInput.SetValue("select id from items")

	m, cmd := m.handleSQLKey(keyMsg("enter"))
	m, _ = update(m, runCmd(cmd))
	if !m.queryActive || !m.queryStreaming || len(client.queryRequests) != 1 {
		t.Fatalf("initial query stream state = active %t, streaming %t, requests %#v", m.queryActive, m.queryStreaming, client.queryRequests)
	}
	if got := m.rows; len(got) != 2 || got[0][0] != "1" || got[1][0] != "2" || !m.hasNextPage {
		t.Fatalf("first query page = %#v, hasNext %t", got, m.hasNextPage)
	}

	m, cmd = m.changePage(+1)
	m, _ = update(m, runCmd(cmd))
	if len(client.queryRequests) != 1 || m.page != 1 || m.rows[0][0] != "3" {
		t.Fatalf("forward query page = requests %d, page %d, rows %#v", len(client.queryRequests), m.page, m.rows)
	}

	m, cmd = m.changePage(-1)
	m, _ = update(m, runCmd(cmd))
	if len(client.queryRequests) != 2 || !client.queryStreams[0].closed || m.page != 0 || m.rows[0][0] != "1" {
		t.Fatalf("previous query page = requests %d, first closed %t, page %d, rows %#v", len(client.queryRequests), client.queryStreams[0].closed, m.page, m.rows)
	}
	request, err := m.exportRequest("query.json")
	if err != nil || request.query == nil || request.query.SQL != "select id from items" {
		t.Fatalf("streamed query export request = %#v, error %v", request, err)
	}
	active := client.queryStreams[1]
	m, cmd = update(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	if cmd == nil || !active.closed {
		t.Fatalf("resizing did not reset the query stream: command %t, closed %t", cmd != nil, active.closed)
	}
}

func TestMutatingQueryDoesNotUseReplayableStream(t *testing.T) {
	client := &queryStreamFakeDriver{fakeDriver: fakeDriver{queryResult: db.Result{Columns: []string{"id"}, Rows: [][]any{{1}}}}}
	m := New(Options{Client: client, Timeout: time.Second})
	m.sqlInput.SetValue("insert into items default values returning id")

	m, cmd := m.handleSQLKey(keyMsg("enter"))
	m, _ = update(m, runCmd(cmd))
	if len(client.queryRequests) != 0 || !m.queryActive || m.queryStreaming {
		t.Fatalf("mutating query state = requests %#v, active %t, streaming %t", client.queryRequests, m.queryActive, m.queryStreaming)
	}
}

func TestIsStreamableQuery(t *testing.T) {
	for _, test := range []struct {
		sql  string
		want bool
	}{
		{sql: "select 1", want: true},
		{sql: "/* comment */ -- another\n SELECT 1", want: true},
		{sql: "with rows as (select 1) select * from rows", want: false},
		{sql: "update items set name = 'x' returning id", want: false},
		{sql: "explain analyze update items set name = 'x'", want: false},
	} {
		if got := isStreamableQuery(test.sql); got != test.want {
			t.Errorf("isStreamableQuery(%q) = %t, want %t", test.sql, got, test.want)
		}
	}
}

func TestStaleTableStreamResultIsClosedAndCancelled(t *testing.T) {
	m := testModel()
	m.loadID = 2
	stream := &fakeRowStream{}
	cancelled := false
	m, _ = update(m, tableStreamRowsLoadedMsg{
		requestID: 1,
		stream:    stream,
		cancel:    func() { cancelled = true },
	})
	if !stream.closed || !cancelled {
		t.Fatalf("stale stream cleanup = closed %t, cancelled %t", stream.closed, cancelled)
	}
}

func TestExportClosesActiveStreamBeforeStarting(t *testing.T) {
	m := New(Options{Client: &fakeDriver{}, ExportDirectory: t.TempDir(), Timeout: time.Second})
	m.columns = []db.Column{{Name: "id"}}
	m.queryActive = true
	m.queryRows = [][]string{{"1"}}
	stream := &fakeRowStream{}
	m.tableStream = tableStreamState{stream: stream}
	m, _ = m.beginJSONExport()

	m, cmd := m.handleExportKey(keyMsg("enter"))
	if cmd == nil || !stream.closed {
		t.Fatalf("export stream cleanup = command %t, closed %t", cmd != nil, stream.closed)
	}
}

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	model, cmd := m.Update(msg)
	return model.(Model), cmd
}

func TestRowsUseComputedPageSize(t *testing.T) {
	f := &fakeDriver{
		schemas: []db.Schema{{Name: "public"}},
		tables:  []db.Table{{Schema: "public", Name: "t"}},
		cols:    []db.Column{{Name: "c"}},
		rows:    makeRows(100),
	}
	m := New(Options{Client: f, Timeout: time.Second})

	// Window size arrives first (height 20 -> page size 12), then tables load.
	var cmd tea.Cmd
	m, cmd = update(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	if m.pageSize != 12 {
		t.Fatalf("want page size 12, got %d", m.pageSize)
	}
	m, cmd = update(m, tablesLoadedMsg{tables: f.tables})
	if msg := runCmd(cmd); msg != nil {
		m, cmd = update(m, msg)
		_ = cmd
	}
	if f.lastLimit != 13 {
		t.Fatalf("table loaded after resize: want limit 13, got %d", f.lastLimit)
	}

	// Reverse order: tables load with the default page size, then a resize
	// should reload with the computed page size.
	m2 := New(Options{Client: f, Timeout: time.Second})
	f.lastLimit = 0
	m2, cmd = update(m2, tablesLoadedMsg{tables: f.tables})
	if msg := runCmd(cmd); msg != nil {
		m2, cmd = update(m2, msg)
		_ = cmd
	}
	if f.lastLimit != maxPageSize+1 {
		t.Fatalf("before resize: want limit %d, got %d", maxPageSize+1, f.lastLimit)
	}
	m2, cmd = update(m2, tea.WindowSizeMsg{Width: 100, Height: 20})
	if cmd == nil {
		t.Fatal("expected reload after resize")
	}
	_ = runCmd(cmd)
	if f.lastLimit != 13 {
		t.Fatalf("after resize: want limit 13, got %d", f.lastLimit)
	}
}

func TestToggleDescriptions(t *testing.T) {
	f := &fakeDriver{
		schemas: []db.Schema{{Name: "public"}},
		tables:  []db.Table{{Schema: "public", Name: "t"}},
		cols:    []db.Column{{Name: "c"}},
		rows:    makeRows(5),
		describeCols: []db.Column{
			{Name: "id", DataType: "integer", IsPrimary: true},
			{Name: "name", DataType: "text", Nullable: true},
		},
	}
	m := New(Options{Client: f, Timeout: time.Second})
	var cmd tea.Cmd
	m, cmd = update(m, tablesLoadedMsg{tables: f.tables})
	runCmd(cmd)

	if m.mode != modeValues {
		t.Fatalf("want default mode values, got %v", m.mode)
	}

	// d -> descriptions
	m, cmd = update(m, keyMsg("d"))
	if m.mode != modeDescriptions {
		t.Fatalf("want mode descriptions after d, got %v", m.mode)
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if len(m.tableInfo.Columns) != 2 {
		t.Fatalf("want 2 described columns, got %d", len(m.tableInfo.Columns))
	}
	if f.lastDescribe != "t" {
		t.Fatalf("DescribeTable called with %q, want t", f.lastDescribe)
	}

	// y -> back to values
	m, cmd = update(m, keyMsg("y"))
	if m.mode != modeValues {
		t.Fatalf("want mode values after y, got %v", m.mode)
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if len(m.rows) != 5 {
		t.Fatalf("want 5 rows back, got %d", len(m.rows))
	}
}

func TestDescriptionsModeLoadsOnSelection(t *testing.T) {
	f := &fakeDriver{
		schemas: []db.Schema{{Name: "public"}},
		tables:  []db.Table{{Schema: "public", Name: "a"}, {Schema: "public", Name: "b"}},
		cols:    []db.Column{{Name: "c"}},
		rows:    makeRows(3),
	}
	m := New(Options{Client: f, Timeout: time.Second})
	var cmd tea.Cmd
	m, cmd = update(m, tablesLoadedMsg{tables: f.tables})
	runCmd(cmd)

	m, cmd = update(m, keyMsg("d"))
	runCmd(cmd)

	f.lastDescribe = ""
	m, cmd = update(m, keyMsg("j"))
	if cmd == nil {
		t.Fatal("expected a load after moving in descriptions mode")
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if f.lastDescribe != "b" {
		t.Fatalf("expected DescribeTable for b, got %q", f.lastDescribe)
	}
	if m.mode != modeDescriptions {
		t.Fatalf("expected to stay in descriptions mode, got %v", m.mode)
	}
}

func TestSchemaSelectionLoadsSelectedSchema(t *testing.T) {
	f := &fakeDriver{
		schemas: []db.Schema{{Name: "analytics"}, {Name: "public"}},
		tables:  []db.Table{{Schema: "public", Name: "events"}},
	}
	m := New(Options{Client: f, Timeout: time.Second})

	var cmd tea.Cmd
	m, cmd = update(m, schemasLoadedMsg{schemas: f.schemas})
	if msg := runCmd(cmd); msg != nil {
		_, _ = update(m, msg)
	}
	if f.lastSchema != "public" {
		t.Fatalf("initial schema = %q, want public", f.lastSchema)
	}

	m.activeOverlay = overlaySchemaPicker
	m.schema = 0
	m, cmd = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if msg := runCmd(cmd); msg != nil {
		_, _ = update(m, msg)
	}
	if f.lastSchema != "analytics" {
		t.Fatalf("selected schema = %q, want analytics", f.lastSchema)
	}
}

func TestTableFilterMatchesTableNames(t *testing.T) {
	m := testModel()
	m.allTables = []db.Table{
		{Schema: "public", Name: "accounts"},
		{Schema: "public", Name: "audit_log"},
		{Schema: "public", Name: "projects"},
	}
	m.filterInput.SetValue("AUD")
	m.applyFilter()
	if len(m.tables) != 1 || m.tables[0].Name != "audit_log" {
		t.Fatalf("filtered tables = %#v, want audit_log", m.tables)
	}
}

func TestSQLQueryPagesResults(t *testing.T) {
	f := &fakeDriver{queryResult: db.Result{
		Columns:      []string{"id", "name"},
		Rows:         [][]any{{1, "one"}, {2, "two"}, {3, "three"}},
		RowsAffected: 3,
		DurationMs:   4,
	}}
	m := New(Options{Client: f, Timeout: time.Second})
	m.pageSize = 2
	m.activeOverlay = overlaySQL
	m.sqlInput.SetValue("select * from things")
	m, cmd := m.handleSQLKey(tea.KeyMsg{Type: tea.KeyEnter})
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if f.lastQuery != "select * from things" || !m.queryActive {
		t.Fatalf("query = %q, active = %t", f.lastQuery, m.queryActive)
	}
	if len(m.rows) != 2 || m.rows[0][0] != "1" {
		t.Fatalf("first query page = %#v", m.rows)
	}
	m, _ = m.changePage(+1)
	if m.page != 1 || len(m.rows) != 1 || m.rows[0][0] != "3" {
		t.Fatalf("second query page = %#v", m.rows)
	}
}

func TestRowCountUpdatesStatus(t *testing.T) {
	f := &fakeDriver{
		tables: []db.Table{{Schema: "public", Name: "events"}},
		count:  42,
	}
	m := New(Options{Client: f, Timeout: time.Second})
	m.tables = f.tables
	m.rows = [][]string{{"value"}}
	m, _ = m.handleCountLoaded(countLoadedMsg{table: f.tables[0], count: f.count})
	if !strings.Contains(m.status, "of 42") {
		t.Fatalf("status = %q, want row count", m.status)
	}
}

func TestQueryResizeRepagesWithoutLoadingTableRows(t *testing.T) {
	m := New(Options{Client: &fakeDriver{}, Timeout: time.Second})
	m.queryActive = true
	m.queryRows = makeRows(10)
	m.page = 1
	m.pageSize = 5
	m, cmd := m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 10})
	if cmd != nil {
		t.Fatal("query resize should page local results without loading table rows")
	}
	if len(m.rows) != 2 {
		t.Fatalf("query rows after resize = %d, want 2", len(m.rows))
	}
}

func TestTablePaginationStopsOnFinalPage(t *testing.T) {
	m := testModel()
	m.pageSize = 2
	m, _ = m.handleRowsLoaded(rowsLoadedMsg{
		columns: []db.Column{{Name: "value"}},
		rows:    makeRows(2),
	})
	if m.hasNextPage {
		t.Fatal("expected no next page without an extra fetched row")
	}
	if got, cmd := m.changePage(+1); got.page != 0 || cmd != nil {
		t.Fatalf("next page should be unavailable: page=%d cmd=%v", got.page, cmd)
	}
}
