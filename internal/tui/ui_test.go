package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func testModel() Model {
	m := New(Options{Client: nil, Timeout: 0})
	m.tables = []db.Table{{Schema: "public", Name: "a"}, {Schema: "public", Name: "b"}}
	m.loading = false
	return m
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func TestMoveSelectionBounds(t *testing.T) {
	m := testModel()
	m.selected = 1
	if got, _ := m.moveSelection(+1); got.selected != 1 {
		t.Fatalf("expected to stay at last, got %d", got.selected)
	}
	m.selected = 0
	if got, _ := m.moveSelection(-1); got.selected != 0 {
		t.Fatalf("expected to stay at first, got %d", got.selected)
	}
	m.selected = 0
	if got, _ := m.moveSelection(+1); got.selected != 1 {
		t.Fatalf("expected to move to 1, got %d", got.selected)
	}
}

func TestChangePageBounds(t *testing.T) {
	m := testModel()
	m.page = 0
	if got, _ := m.changePage(-1); got.page != 0 {
		t.Fatalf("expected page to stay 0, got %d", got.page)
	}
	m.page = 2
	if got, _ := m.changePage(-1); got.page != 1 {
		t.Fatalf("expected page 1, got %d", got.page)
	}
}

func TestUpdateRoutesWindowSize(t *testing.T) {
	got, _ := testModel().Update(tea.WindowSizeMsg{Width: 80})
	if got.(Model).width != 80 {
		t.Fatalf("expected width 80, got %d", got.(Model).width)
	}
}

func TestUpdateQuitKey(t *testing.T) {
	_, cmd := testModel().Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestHelpOverlay(t *testing.T) {
	m := testModel()
	m, _ = update(m, keyMsg("?"))
	if !m.showHelp {
		t.Fatal("expected help overlay to open")
	}
	if got := m.View(); !strings.Contains(got, "Keyboard shortcuts") {
		t.Fatalf("help overlay missing heading: %q", got)
	}
	m, _ = update(m, keyMsg("x"))
	if m.showHelp {
		t.Fatal("expected any key to close help overlay")
	}
}

func TestHandlersReportErrors(t *testing.T) {
	m := testModel()
	got, _ := m.handleTablesLoaded(tablesLoadedMsg{err: errBoom})
	if got.status != "failed to load tables: boom" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	got, _ = m.handleRowsLoaded(rowsLoadedMsg{err: errBoom})
	if got.status != "rows error: boom" {
		t.Fatalf("unexpected status: %q", got.status)
	}
}

func TestKeepInView(t *testing.T) {
	if got := keepInView(2, 0, 4, 7); got != 0 {
		t.Fatalf("selected within window, want offset 0, got %d", got)
	}
	if got := keepInView(4, 0, 4, 7); got != 1 {
		t.Fatalf("selection moved past bottom edge, want offset 1, got %d", got)
	}
	if got := keepInView(6, 3, 4, 7); got != 3 {
		t.Fatalf("selection at end, want offset 3, got %d", got)
	}
	if got := keepInView(1, 3, 4, 7); got != 1 {
		t.Fatalf("selection moved above window, want offset 1, got %d", got)
	}
	if got := keepInView(3, 0, 4, 3); got != 0 {
		t.Fatalf("list shorter than window, want offset 0, got %d", got)
	}
}

func TestScrollWithMoveSelection(t *testing.T) {
	m := testModel()
	m.tables = make([]db.Table, 7)
	for i := range m.tables {
		m.tables[i] = db.Table{Schema: "public", Name: "t"}
	}
	m.selected = 3
	for i := 0; i < 3; i++ {
		m, _ = m.moveSelection(+1)
	}
	if m.selected != 6 {
		t.Fatalf("want selected 6, got %d", m.selected)
	}
	if m.scroll != 3 {
		t.Fatalf("want scroll 3 at bottom edge, got %d", m.scroll)
	}
	for i := 0; i < 6; i++ {
		m, _ = m.moveSelection(-1)
	}
	if m.selected != 0 {
		t.Fatalf("want selected 0, got %d", m.selected)
	}
	if m.scroll != 0 {
		t.Fatalf("want scroll 0 at top, got %d", m.scroll)
	}
}

func TestComputedPageSize(t *testing.T) {
	m := testModel()
	m.height = 40
	if got := m.computedPageSize(); got != 32 {
		t.Fatalf("height 40: want page size 32, got %d", got)
	}
	m.height = 120
	if got := m.computedPageSize(); got != maxPageSize {
		t.Fatalf("tall screen: want page size %d, got %d", maxPageSize, got)
	}
	m.height = 4
	if got := m.computedPageSize(); got != 1 {
		t.Fatalf("tiny screen: want page size 1, got %d", got)
	}
}

func TestWindowSizeUpdatesPageSize(t *testing.T) {
	m := testModel()
	m.height = 0
	m.pageSize = maxPageSize

	m, _ = m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.pageSize != 32 {
		t.Fatalf("want page size 32, got %d", m.pageSize)
	}
}

func TestWindowSizeReloadsRows(t *testing.T) {
	m := testModel()
	m.height = 40
	m, _ = m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.pageSize != 32 {
		t.Fatalf("want page size 32, got %d", m.pageSize)
	}
	if _, cmd := m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40}); cmd != nil {
		t.Fatal("expected no reload when page size is unchanged")
	}
	m.pageSize = 0
	if _, cmd := m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40}); cmd == nil {
		t.Fatal("expected reload when page size changes")
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }

var errBoom error = testErr("boom")
