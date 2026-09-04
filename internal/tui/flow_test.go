package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

type fakeDriver struct {
	schemas       []db.Schema
	tables        []db.Table
	cols          []db.Column
	rows          [][]string
	describeCols  []db.Column
	lastLimit     int
	lastOffset    int
	lastSchema    string
	lastDescribe  string
	describeCalls int
	count         int64
	queryResult   db.Result
	lastQuery     string
}

func (f *fakeDriver) DbType() db.DbType                               { return db.DbTypePostgres }
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
	rows := f.rows
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return f.cols, rows, nil
}
func (f *fakeDriver) CountRows(context.Context, db.Table) (int64, error) { return f.count, nil }

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

	m.showSchemas = true
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
	m.sqlMode = true
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
