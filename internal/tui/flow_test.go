package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

type fakeDriver struct {
	tables        []db.Table
	cols          []db.Column
	rows          [][]string
	describeCols  []db.Column
	lastLimit     int
	lastOffset    int
	lastDescribe  string
	describeCalls int
}

func (f *fakeDriver) DbType() db.DbType                               { return db.DbTypePostgres }
func (f *fakeDriver) Connect(context.Context, db.ConnectConfig) error { return nil }
func (f *fakeDriver) Close() error                                    { return nil }
func (f *fakeDriver) Ping(context.Context) error                      { return nil }
func (f *fakeDriver) ListSchemas(context.Context) ([]db.Schema, error) {
	return nil, nil
}
func (f *fakeDriver) ListTables(context.Context, string) ([]db.Table, error) {
	return f.tables, nil
}
func (f *fakeDriver) DescribeTable(_ context.Context, schema, table string) (db.TableInfo, error) {
	f.lastDescribe = table
	f.describeCalls++
	return db.TableInfo{Schema: schema, Name: table, Columns: f.describeCols}, nil
}
func (f *fakeDriver) Query(context.Context, db.Query) (db.Result, error) { return db.Result{}, nil }
func (f *fakeDriver) Rows(ctx context.Context, tbl db.Table, limit, offset int) ([]db.Column, [][]string, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	rows := f.rows
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return f.cols, rows, nil
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

func update(m Model, msg tea.Msg) (Model, tea.Cmd) {
	model, cmd := m.Update(msg)
	return model.(Model), cmd
}

func TestRowsUseComputedPageSize(t *testing.T) {
	f := &fakeDriver{
		tables: []db.Table{{Schema: "public", Name: "t"}},
		cols:   []db.Column{{Name: "c"}},
		rows:   makeRows(100),
	}
	m := New(Options{Client: f, Timeout: time.Second})

	// Window size arrives first (height 20 -> page size 11), then tables load.
	var cmd tea.Cmd
	m, cmd = update(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	if m.pageSize != 11 {
		t.Fatalf("want page size 11, got %d", m.pageSize)
	}
	m, cmd = update(m, tablesLoadedMsg{tables: f.tables})
	if msg := runCmd(cmd); msg != nil {
		m, cmd = update(m, msg)
		_ = cmd
	}
	if f.lastLimit != 11 {
		t.Fatalf("table loaded after resize: want limit 11, got %d", f.lastLimit)
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
	if f.lastLimit != maxPageSize {
		t.Fatalf("before resize: want default limit %d, got %d", maxPageSize, f.lastLimit)
	}
	m2, cmd = update(m2, tea.WindowSizeMsg{Width: 100, Height: 20})
	if cmd == nil {
		t.Fatal("expected reload after resize")
	}
	_ = runCmd(cmd)
	if f.lastLimit != 11 {
		t.Fatalf("after resize: want limit 11, got %d", f.lastLimit)
	}
}

func TestToggleDescriptions(t *testing.T) {
	f := &fakeDriver{
		tables: []db.Table{{Schema: "public", Name: "t"}},
		cols:   []db.Column{{Name: "c"}},
		rows:   makeRows(5),
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
		tables: []db.Table{{Schema: "public", Name: "a"}, {Schema: "public", Name: "b"}},
		cols:   []db.Column{{Name: "c"}},
		rows:   makeRows(3),
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
