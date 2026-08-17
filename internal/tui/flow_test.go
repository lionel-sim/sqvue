package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

type fakeDriver struct {
	tables     []db.Table
	cols       []db.Column
	rows       [][]string
	lastLimit  int
	lastOffset int
}

func (f *fakeDriver) DbType() db.DbType                                { return db.DbTypePostgres }
func (f *fakeDriver) Connect(context.Context, db.ConnectConfig) error  { return nil }
func (f *fakeDriver) Close() error                                     { return nil }
func (f *fakeDriver) Ping(context.Context) error                       { return nil }
func (f *fakeDriver) ListSchemas(context.Context) ([]db.Schema, error) { return nil, nil }
func (f *fakeDriver) ListTables(context.Context, string) ([]db.Table, error) {
	return f.tables, nil
}
func (f *fakeDriver) DescribeTable(context.Context, string, string) (db.TableInfo, error) {
	return db.TableInfo{}, nil
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
