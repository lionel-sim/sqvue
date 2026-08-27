package tui

import (
	"strings"
	"testing"

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

func strPtr(s string) *string { return &s }
