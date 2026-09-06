package tui

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

const exportBatchSize = 500

type csvExportRequest struct {
	path           string
	columns        []db.Column
	visibleColumns []bool
	queryRows      [][]string
	table          *db.Table
	filters        []db.RowFilter
}

func (m Model) beginCSVExport() (Model, tea.Cmd) {
	if m.mode != modeValues || len(m.columns) == 0 {
		m.status = "CSV export is unavailable for this view"
		return m, nil
	}
	m.exportInput.SetValue(m.defaultCSVExportPath(time.Now()))
	m.activeOverlay = overlayExportCSV
	m.exportInput.Focus()
	return m, nil
}

func (m Model) handleCSVExportKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.activeOverlay = overlayNone
		m.exportInput.Blur()
		m.restoreBrowseStatus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		path := strings.TrimSpace(m.exportInput.Value())
		if path == "" {
			m.status = "provide a CSV export path"
			return m, nil
		}
		request, err := m.csvExportRequest(path)
		if err != nil {
			return m.fail("CSV export failed", err)
		}
		m.activeOverlay = overlayNone
		m.exportInput.Blur()
		m.exportID++
		m.status = "exporting CSV..."
		return m, writeCSVCmd(m.client, m.timeout, m.exportID, request)
	}
	var cmd tea.Cmd
	m.exportInput, cmd = m.exportInput.Update(msg)
	return m, cmd
}

func (m Model) csvExportRequest(path string) (csvExportRequest, error) {
	request := csvExportRequest{
		path:           path,
		columns:        append([]db.Column(nil), m.columns...),
		visibleColumns: append([]bool(nil), m.visibleColumns...),
		filters:        append([]db.RowFilter(nil), m.browseFilters...),
	}
	if m.queryActive {
		request.queryRows = append([][]string(nil), m.queryRows...)
		return request, nil
	}
	table := m.currentTable()
	if table == nil {
		return csvExportRequest{}, fmt.Errorf("no table selected")
	}
	copy := *table
	request.table = &copy
	return request, nil
}

func (m Model) defaultCSVExportPath(now time.Time) string {
	directory := m.exportDirectory
	if directory == "" {
		directory = "."
	}
	name := "sqvue-query"
	if !m.queryActive {
		if table := m.currentTable(); table != nil {
			name = "sqvue-" + safeExportName(table.Schema) + "-" + safeExportName(table.Name)
		}
	}
	return filepath.Join(directory, name+"-"+now.Format("20060102-150405")+".csv")
}

func safeExportName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "results"
	}
	return b.String()
}

func writeCSVCmd(client db.Driver, timeout time.Duration, exportID uint64, request csvExportRequest) tea.Cmd {
	return func() tea.Msg {
		rows, err := writeCSV(client, timeout, request)
		return csvExportedMsg{exportID: exportID, path: request.path, rows: rows, err: err}
	}
}

func writeCSV(client db.Driver, timeout time.Duration, request csvExportRequest) (rows int, returnErr error) {
	directory := filepath.Dir(request.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return 0, fmt.Errorf("create export directory: %w", err)
	}
	file, err := os.OpenFile(request.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, fmt.Errorf("create CSV file: %w", err)
	}
	defer func() {
		if err := file.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close CSV file: %w", err)
		}
	}()

	writer := csv.NewWriter(file)
	if err := writer.Write(visibleColumnNames(request.columns, request.visibleColumns)); err != nil {
		return 0, fmt.Errorf("write CSV header: %w", err)
	}
	writeRows := func(values [][]string) error {
		for _, row := range values {
			if err := writer.Write(visibleCSVRow(row, request.visibleColumns)); err != nil {
				return err
			}
			rows++
		}
		return nil
	}
	if request.table == nil {
		if err := writeRows(request.queryRows); err != nil {
			return 0, fmt.Errorf("write CSV rows: %w", err)
		}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		for offset := 0; ; offset += exportBatchSize {
			values, err := exportTableRows(ctx, client, *request.table, request.filters, offset)
			if err != nil {
				return 0, fmt.Errorf("load rows for CSV export: %w", err)
			}
			if err := writeRows(values); err != nil {
				return 0, fmt.Errorf("write CSV rows: %w", err)
			}
			if len(values) < exportBatchSize {
				break
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return 0, fmt.Errorf("write CSV: %w", err)
	}
	return rows, nil
}

func exportTableRows(ctx context.Context, client db.Driver, table db.Table, filters []db.RowFilter, offset int) ([][]string, error) {
	if len(filters) == 0 {
		_, rows, err := client.Rows(ctx, table, exportBatchSize, offset)
		return rows, err
	}
	_, rows, err := client.BrowseRows(ctx, db.BrowseRequest{
		Table: table, Limit: exportBatchSize, Offset: offset, Filters: filters,
	})
	return rows, err
}

func visibleColumnNames(columns []db.Column, visible []bool) []string {
	names := make([]string, 0, len(columns))
	for i, column := range columns {
		if len(visible) == 0 || (i < len(visible) && visible[i]) {
			names = append(names, column.Name)
		}
	}
	return names
}

func visibleCSVRow(row []string, visible []bool) []string {
	if len(visible) == 0 {
		return append([]string(nil), row...)
	}
	values := make([]string, 0, len(row))
	for i, value := range row {
		if i < len(visible) && visible[i] {
			values = append(values, value)
		}
	}
	return values
}
