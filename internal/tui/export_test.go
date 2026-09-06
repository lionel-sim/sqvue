package tui

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"sqvue/internal/db"
)

func TestWriteCSVExportsVisibleQueryColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "query.csv")
	rows, err := writeCSV(nil, time.Second, csvExportRequest{
		path: path,
		columns: []db.Column{
			{Name: "id"}, {Name: "secret"}, {Name: "note"},
		},
		visibleColumns: []bool{true, false, true},
		queryRows:      [][]string{{"1", "hidden", "a, \"quoted\" note"}},
	})
	if err != nil {
		t.Fatalf("writeCSV() error = %v", err)
	}
	if rows != 1 {
		t.Fatalf("writeCSV() rows = %d, want 1", rows)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	want := [][]string{{"id", "note"}, {"1", "a, \"quoted\" note"}}
	if len(records) != len(want) {
		t.Fatalf("CSV records = %#v, want %#v", records, want)
	}
	for i := range want {
		if got := records[i]; len(got) != len(want[i]) || got[0] != want[i][0] || got[1] != want[i][1] {
			t.Fatalf("CSV record %d = %#v, want %#v", i, got, want[i])
		}
	}
}

func TestWriteCSVExportsAllFilteredTableRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table.csv")
	client := &fakeDriver{cols: []db.Column{{Name: "id"}, {Name: "name"}}}
	client.rows = make([][]string, exportBatchSize+1)
	for i := range client.rows {
		client.rows[i] = []string{strconv.Itoa(i), "person"}
	}
	filter := db.RowFilter{Column: "name", Operator: db.FilterContains, Value: "a"}
	rows, err := writeCSV(client, time.Second, csvExportRequest{
		path:           path,
		columns:        client.cols,
		visibleColumns: []bool{true, true},
		table:          &db.Table{Schema: "public", Name: "people"},
		filters:        []db.RowFilter{filter},
	})
	if err != nil {
		t.Fatalf("writeCSV() error = %v", err)
	}
	if rows != exportBatchSize+1 {
		t.Fatalf("writeCSV() rows = %d, want %d", rows, exportBatchSize+1)
	}
	if got := client.lastBrowse; got.Table.Name != "people" || got.Limit != exportBatchSize || got.Offset != exportBatchSize || len(got.Filters) != 1 || got.Filters[0] != filter {
		t.Fatalf("BrowseRows() request = %#v", got)
	}
}

func TestWriteJSONExportsVisibleQueryColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "query.json")
	rows, err := writeJSON(nil, time.Second, exportRequest{
		path: path,
		columns: []db.Column{
			{Name: "id"}, {Name: "secret"}, {Name: "note"},
		},
		visibleColumns: []bool{true, false, true},
		queryRows:      [][]string{{"1", "hidden", "a \"quoted\" note"}},
	})
	if err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}
	if rows != 1 {
		t.Fatalf("writeJSON() rows = %d, want 1", rows)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var records []map[string]string
	if err := json.Unmarshal(contents, &records); err != nil {
		t.Fatalf("JSON export is invalid: %v", err)
	}
	want := []map[string]string{{"id": "1", "note": "a \"quoted\" note"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("JSON records = %#v, want %#v", records, want)
	}
}

func TestWriteJSONExportsAllFilteredTableRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "table.json")
	client := &fakeDriver{cols: []db.Column{{Name: "id"}, {Name: "name"}}}
	client.rows = make([][]string, exportBatchSize+1)
	for i := range client.rows {
		client.rows[i] = []string{strconv.Itoa(i), "person"}
	}
	filter := db.RowFilter{Column: "name", Operator: db.FilterContains, Value: "a"}
	rows, err := writeJSON(client, time.Second, exportRequest{
		path:           path,
		columns:        client.cols,
		visibleColumns: []bool{true, true},
		table:          &db.Table{Schema: "public", Name: "people"},
		filters:        []db.RowFilter{filter},
	})
	if err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}
	if rows != exportBatchSize+1 {
		t.Fatalf("writeJSON() rows = %d, want %d", rows, exportBatchSize+1)
	}
	if got := client.lastBrowse; got.Table.Name != "people" || got.Limit != exportBatchSize || got.Offset != exportBatchSize || len(got.Filters) != 1 || got.Filters[0] != filter {
		t.Fatalf("BrowseRows() request = %#v", got)
	}
}

func TestCSVExportPromptUsesConfiguredDirectory(t *testing.T) {
	m := New(Options{ExportDirectory: "/exports"})
	m.tables = []db.Table{{Schema: "public", Name: "order items"}}
	m.columns = []db.Column{{Name: "id"}}
	path := m.defaultCSVExportPath(time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC))
	if want := "/exports/sqvue-public-order_items-20260906-010203.csv"; path != want {
		t.Fatalf("defaultCSVExportPath() = %q, want %q", path, want)
	}
	m, _ = m.beginCSVExport()
	if m.activeOverlay != overlayExport {
		t.Fatalf("active overlay = %v, want CSV export prompt", m.activeOverlay)
	}
}

func TestJSONExportPromptUsesConfiguredDirectory(t *testing.T) {
	m := New(Options{ExportDirectory: "/exports"})
	m.tables = []db.Table{{Schema: "public", Name: "order items"}}
	m.columns = []db.Column{{Name: "id"}}
	path := m.defaultJSONExportPath(time.Date(2026, 9, 6, 1, 2, 3, 0, time.UTC))
	if want := "/exports/sqvue-public-order_items-20260906-010203.json"; path != want {
		t.Fatalf("defaultJSONExportPath() = %q, want %q", path, want)
	}
	m, _ = m.beginJSONExport()
	if m.activeOverlay != overlayExport || m.exportFormat != exportJSON {
		t.Fatalf("export prompt = (%v, %q), want JSON export prompt", m.activeOverlay, m.exportFormat)
	}
	if m.exportInput.Prompt != "Export JSON to: " {
		t.Fatalf("export prompt = %q, want JSON prompt", m.exportInput.Prompt)
	}
}

func TestJSONExportRunsInBackgroundAndReportsStatus(t *testing.T) {
	m := New(Options{ExportDirectory: t.TempDir(), Timeout: time.Second})
	m.columns = []db.Column{{Name: "id"}}
	m.queryActive = true
	m.queryRows = [][]string{{"1"}}

	m, _ = update(m, keyMsg("E"))
	if m.activeOverlay != overlayExport || m.status == "exporting JSON..." {
		t.Fatalf("JSON export prompt = (%v, %q), want open prompt", m.activeOverlay, m.status)
	}
	m, cmd := update(m, keyMsg("enter"))
	if m.status != "exporting JSON..." {
		t.Fatalf("status = %q, want JSON export progress", m.status)
	}
	m, _ = update(m, runCmd(cmd))
	if !strings.HasPrefix(m.status, "exported 1 rows to ") || m.lastErr != nil {
		t.Fatalf("status = %q, error = %v, want successful JSON export", m.status, m.lastErr)
	}
}
