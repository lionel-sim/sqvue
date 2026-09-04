package tui

import (
	"strings"
	"testing"

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
		{Name: "name", DataType: "text", Nullable: true, Default: strPtr("'anon'")},
	}
	var b strings.Builder
	renderDescriptions(&b, cols, 60, -1)

	out := b.String()
	for _, want := range []string{"column", "type", "nullable", "default", "primary", "id", "integer", "yes", "name", "'anon'"} {
		if !strings.Contains(out, want) {
			t.Fatalf("descriptions output missing %q:\n%s", want, out)
		}
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

func TestHelpRendersAsModalOverCurrentView(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 20
	m.status = "public.accounts page 1"
	m.showHelp = true

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
