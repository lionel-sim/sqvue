package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
	"sqvue/internal/theme"
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

func TestBrowseFilterPickerFitsTerminalAndShowsFocusedBindings(t *testing.T) {
	m := testModel()
	m.width, m.height = 80, 8
	m.focused = true
	m.activeOverlay = overlayBrowseFilterOperator
	m.browseFilterColumn = "name"
	m.client = &fakeDriver{}
	view := m.View()
	if len(strings.Split(view, "\n")) != m.height {
		t.Fatalf("picker rendered %d lines for height %d", len(strings.Split(view, "\n")), m.height)
	}
	if !strings.Contains(view, "/ filter rows | x clear") {
		t.Fatalf("focused footer = %q", view)
	}
}

func TestRenderVisibleRowsHighlightsActiveRow(t *testing.T) {
	columns := []db.Column{{Name: "name"}}
	rows := [][]string{{"books"}, {"games"}}
	var b strings.Builder
	renderVisibleRows(&b, columns, rows, rowRenderOptions{width: 20, maxRows: -1, activeRow: 1, activeColumn: 0})

	want := theme.ActiveCell.Render(formatRow(rows[1], layoutColumns(20, columns, rows)))
	if !strings.Contains(b.String(), want) {
		t.Fatalf("active row is not highlighted:\n%s", b.String())
	}
}

func TestRenderVisibleRowsHighlightsActiveCell(t *testing.T) {
	columns := []db.Column{{Name: "id"}, {Name: "name"}}
	rows := [][]string{{"1", "books"}}
	var b strings.Builder
	renderVisibleRows(&b, columns, rows, rowRenderOptions{width: 20, maxRows: -1, activeRow: 0, activeColumn: 1})

	widths := layoutColumns(20, columns, rows)
	want := theme.ActiveCell.Render(formatCell("books", widths[1]))
	if !strings.Contains(b.String(), want) {
		t.Fatalf("active cell is not highlighted:\n%s", b.String())
	}
}

func TestRenderRowDetailShowsFullValue(t *testing.T) {
	m := testModel()
	m.width = 60
	m.height = 20
	m.rows = [][]string{{"a complete customer account name"}}
	m.columns = []db.Column{{Name: "name"}}

	out := ansi.Strip(renderRowDetailModal(m))
	if !strings.Contains(out, "name: a complete customer account name") {
		t.Fatalf("row details missing full value:\n%s", out)
	}
}

func TestWrapDetailValueWrapsLongValues(t *testing.T) {
	if got := strings.Join(wrapDetailValue("abcdef", 3), ","); got != "abc,def" {
		t.Fatalf("wrapped value = %q", got)
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

func TestLayoutColumnsAccountsForForeignKeyIndicator(t *testing.T) {
	columns := []db.Column{{Name: "parent_id", ForeignKey: &db.ForeignKey{Table: "categories", Column: "id"}}}
	width := layoutColumns(40, columns, nil)[0]
	if got := formatCell(columnNames(columns)[0], width); got != "parent_id [FK]" {
		t.Fatalf("foreign-key header = %q", got)
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

func TestRenderFooterShowsGridFocus(t *testing.T) {
	m := testModel()
	m.width = 100
	m.focused = true
	if got := ansi.Strip(renderFooter(m)); !strings.Contains(got, "Focus: rows") {
		t.Fatalf("footer missing grid focus: %q", got)
	}
}

func TestRenderFooterShowsGridCountPrefix(t *testing.T) {
	m := testModel()
	m.width = 100
	m.focused = true
	m.countPrefix = 23
	if got := ansi.Strip(renderFooter(m)); !strings.Contains(got, "Jump: 23") {
		t.Fatalf("footer missing count prefix: %q", got)
	}
}

func TestHelpShowsBindingsForCurrentFocus(t *testing.T) {
	m := testModel()
	m.width = 100
	tableHelp := ansi.Strip(renderHelpDialog(m))
	if !strings.Contains(tableHelp, "show rows") || strings.Contains(tableHelp, "copy cell") {
		t.Fatalf("table help has wrong bindings:\n%s", tableHelp)
	}

	m.focused = true
	gridHelp := ansi.Strip(renderHelpDialog(m))
	if !strings.Contains(gridHelp, "copy cell") || strings.Contains(gridHelp, "show rows") {
		t.Fatalf("grid help has wrong bindings:\n%s", gridHelp)
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
