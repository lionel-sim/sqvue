package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
)

func TestRenderRowsClips(t *testing.T) {
	cols := []db.Column{{Name: "c"}}
	rows := make([][]string, 10)
	for i := range rows {
		rows[i] = []string{"x"}
	}

	var b strings.Builder
	renderRows(&b, cols, rows, 40, 3)
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(lines) != 4 { // column header + 3 data rows
		t.Fatalf("want 4 lines, got %d: %q", len(lines), lines)
	}

	b.Reset()
	renderRows(&b, cols, rows, 40, -1)
	lines = strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(lines) != 11 {
		t.Fatalf("want 11 lines (no clip), got %d", len(lines))
	}
}

func TestRenderDescriptions(t *testing.T) {
	cols := []db.Column{
		{Name: "id", DataType: "integer", IsPrimary: true},
		{Name: "owner_id", DataType: "integer", Nullable: true, Default: strPtr("'anon'"), ForeignKey: &db.ForeignKey{Schema: "public", Table: "customer_accounts", Column: "owner_id"}},
	}
	var b strings.Builder
	renderDescriptions(&b, cols, 80, -1)

	out := b.String()
	for _, want := range []string{"column", "references", "id", "integer", "yes", "owner_id", "'anon'", "public.customer_accounts.owner_id"} {
		if !strings.Contains(out, want) {
			t.Fatalf("descriptions output missing %q:\n%s", want, out)
		}
	}
}

func TestColumnNamesMarksForeignKeys(t *testing.T) {
	names := columnNames([]db.Column{
		{Name: "id"},
		{Name: "owner_id", ForeignKey: &db.ForeignKey{Table: "owners", Column: "id"}},
	})
	if got := strings.Join(names, ","); got != "id,owner_id [FK]" {
		t.Fatalf("column names = %q", got)
	}
}

func TestForeignKeyLabelOmitsEmptyParts(t *testing.T) {
	if got := foreignKeyLabel(&db.ForeignKey{Table: "owners"}); got != "owners" {
		t.Fatalf("foreignKeyLabel() = %q, want owners", got)
	}
}

func TestRenderFooterStaysOnBottomLine(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 20
	m.status = "public.accounts page 1 (2 rows)"

	lines := strings.Split(m.View(), "\n")
	if len(lines) != 20 {
		t.Fatalf("rendered %d lines, want 20", len(lines))
	}
	footer := lines[len(lines)-1]
	if !strings.Contains(footer, "s schema | / filter | j/k navigate | d columns | y rows | q quit") {
		t.Fatalf("footer missing key bindings: %q", footer)
	}
	if !strings.Contains(footer, "Status: public.accounts page 1 (2 rows)") {
		t.Fatalf("footer missing status: %q", footer)
	}
}

func TestRenderTableListUsesSchemaHeaderAndMarksViews(t *testing.T) {
	var b strings.Builder
	renderTableList(&b, "public", []db.Table{
		{Schema: "public", Name: "categories", Type: "table"},
		{Schema: "public", Name: "sales_summary", Type: "view"},
	}, 0, 0, "")

	out := ansi.Strip(b.String())
	if !strings.Contains(out, "Tables: public") {
		t.Fatalf("table list missing schema header: %q", out)
	}
	if !strings.Contains(out, "> categories") {
		t.Fatalf("table list missing bare table name: %q", out)
	}
	if strings.Contains(out, "public.categories") || strings.Contains(out, "[table]") {
		t.Fatalf("table list repeats schema or table marker: %q", out)
	}
	if !strings.Contains(out, "sales_summary [view]") {
		t.Fatalf("table list does not mark views: %q", out)
	}
}

func TestSQLInputUsesAvailableTerminalWidth(t *testing.T) {
	m := testModel()
	m, _ = m.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 20})
	m.activeOverlay = overlaySQL

	out := ansi.Strip(m.View())
	if !strings.Contains(out, "SQL> SELECT * FROM ...") {
		t.Fatalf("SQL input placeholder was truncated: %q", out)
	}
	if m.sqlInput.Width != 75 {
		t.Fatalf("SQL input width = %d, want 75", m.sqlInput.Width)
	}
}

func TestVisibleDataHidesUncheckedColumns(t *testing.T) {
	columns := []db.Column{{Name: "id"}, {Name: "email"}, {Name: "password_hash"}}
	rows := [][]string{{"1", "a@example.com", "secret"}}
	gotColumns, gotRows := visibleData(columns, rows, []bool{true, true, false})
	if len(gotColumns) != 2 || gotColumns[1].Name != "email" {
		t.Fatalf("columns = %#v", gotColumns)
	}
	if len(gotRows) != 1 || strings.Join(gotRows[0], ",") != "1,a@example.com" {
		t.Fatalf("rows = %#v", gotRows)
	}
}

func TestSanitizeTextEscapesTerminalControls(t *testing.T) {
	got := sanitizeText("line\n\x1b]8;;https://example.com\a")
	if strings.Contains(got, "\n") || strings.Contains(got, "\x1b") {
		t.Fatalf("sanitized text still contains terminal controls: %q", got)
	}
	if !strings.Contains(got, `\n`) || !strings.Contains(got, `\x1b`) {
		t.Fatalf("sanitized text = %q", got)
	}
}

func TestHelpRendersAsModalOverCurrentView(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 20
	m.status = "public.accounts page 1"
	m.activeOverlay = overlayHelp

	out := m.View()
	if !strings.Contains(out, "Keyboard shortcuts") {
		t.Fatalf("help dialog missing heading: %q", out)
	}
	if !strings.Contains(out, "Status: public.accounts page 1") {
		t.Fatalf("background footer missing from modal: %q", out)
	}
	if !strings.Contains(out, "┏━ Keyboard shortcuts") {
		t.Fatalf("help dialog missing titled border: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "┏") {
			left := strings.Index(plain, "┏")
			right := len(plain) - len(strings.TrimRight(plain, " "))
			if left != right {
				t.Fatalf("help dialog is not horizontally centered: %q", plain)
			}
		}
	}
	var borderWidths []int
	for _, line := range strings.Split(out, "\n") {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "┏") || strings.Contains(plain, "┗") {
			borderWidths = append(borderWidths, ansi.StringWidth(plain))
		}
	}
	if len(borderWidths) != 2 || borderWidths[0] != borderWidths[1] {
		t.Fatalf("modal border widths = %v, want matching top and bottom widths", borderWidths)
	}
}

func strPtr(s string) *string { return &s }
