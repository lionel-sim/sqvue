package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/config"
)

func TestSQLTextAreaInsertsNewlineAndRunsWithCtrlR(t *testing.T) {
	m := testModel()
	m.activeOverlay = overlaySQL
	m.sqlInput.Focus()
	m.sqlInput.SetValue("select 1")

	m, _ = m.handleSQLKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.sqlInput.Value(); got != "select 1\n" {
		t.Fatalf("SQL after Enter = %q, want a newline", got)
	}

	m.sqlInput.SetValue("select 1")
	m, cmd := m.handleSQLKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	if !m.loading || cmd == nil {
		t.Fatalf("Ctrl+R did not start query: loading %t, command %t", m.loading, cmd != nil)
	}
}

func TestSQLTextAreaArrowsNavigateMultilineEditor(t *testing.T) {
	m := testModel()
	m.queryStore = &config.QueryStore{Profiles: map[string]config.QueryProfile{
		"default": {History: []string{"select from history"}},
	}}
	m.activeOverlay = overlaySQL
	m.sqlInput.Focus()
	m.sqlInput.SetValue("top\nbottom")

	m, _ = m.handleSQLKey(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.handleSQLKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if got := m.sqlInput.Value(); got != "topX\nbottom" {
		t.Fatalf("SQL after Up = %q, want cursor on the previous line", got)
	}

	m, _ = m.handleSQLKey(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.handleSQLKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Y")})
	if got := m.sqlInput.Value(); got != "topX\nbottYom" {
		t.Fatalf("SQL after Down = %q, want cursor on the next line", got)
	}
}

func TestFormatSQLPreservesQuotedText(t *testing.T) {
	got := formatSQL("select 'from  a table', \"where\" from items where note = 'left join'")
	for _, want := range []string{"SELECT 'from  a table', \"where\"", "\nFROM items", "\nWHERE note = 'left join'"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted SQL = %q, missing %q", got, want)
		}
	}
}

func TestExplainRejectsMutationAndPreservesEditor(t *testing.T) {
	m := testModel()
	m.sqlInput.SetValue("update accounts set enabled = true")
	m, cmd := m.explainSQL()
	if cmd != nil || m.status != "EXPLAIN is available only for SELECT queries" {
		t.Fatalf("mutation EXPLAIN = status %q, command %t", m.status, cmd != nil)
	}

	m.sqlInput.SetValue("select * from accounts")
	m, cmd = m.explainSQL()
	if cmd == nil || m.querySourceSQL != "EXPLAIN select * from accounts" || !m.queryPreserveEditor {
		t.Fatalf("SELECT EXPLAIN did not start safely: source %q preserve %t command %t", m.querySourceSQL, m.queryPreserveEditor, cmd != nil)
	}
	m, _ = m.handleQueryLoaded(queryLoadedMsg{requestID: m.loadID, sql: m.querySourceSQL})
	if got := m.sqlInput.Value(); got != "select * from accounts" {
		t.Fatalf("EXPLAIN cleared editor = %q", got)
	}
}

func TestQueryErrorReopensEditorWithLocation(t *testing.T) {
	m := testModel()
	m.loadID = 7
	m.querySourceSQL = "select id\nfrom\nwhere id = 1"
	m.sqlInput.SetValue(m.querySourceSQL)
	m, _ = m.handleQueryLoaded(queryLoadedMsg{requestID: 7, err: errors.New("syntax error at position 20")})
	if m.activeOverlay != overlaySQL || !strings.Contains(m.status, "line 3, column 5") || m.lastErr == nil {
		t.Fatalf("query error state = overlay %v status %q error %v", m.activeOverlay, m.status, m.lastErr)
	}
}
