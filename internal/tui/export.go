package tui

import (
	"context"
	"encoding/csv"
	"encoding/json"
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

type exportFormat string

const (
	exportCSV  exportFormat = "CSV"
	exportJSON exportFormat = "JSON"
)

type exportRequest struct {
	path           string
	columns        []db.Column
	visibleColumns []bool
	queryRows      [][]string
	query          *db.Query
	table          *db.Table
	filters        []db.RowFilter
}

// csvExportRequest remains an alias so the CSV writer can retain its focused API.
type csvExportRequest = exportRequest

func (m Model) beginCSVExport() (Model, tea.Cmd) {
	return m.beginExport(exportCSV)
}

func (m Model) beginJSONExport() (Model, tea.Cmd) {
	return m.beginExport(exportJSON)
}

func (m Model) beginExport(format exportFormat) (Model, tea.Cmd) {
	if m.mode != modeValues || len(m.columns) == 0 {
		m.status = string(format) + " export is unavailable for this view"
		return m, nil
	}
	m.exportFormat = format
	m.exportInput.Prompt = "Export " + string(format) + " to: "
	m.exportInput.Placeholder = "path/to/results." + strings.ToLower(string(format))
	m.exportInput.Width = inputWidth(m.width, m.exportInput.Prompt)
	m.exportInput.SetValue(m.defaultExportPath(time.Now(), format))
	m.activeOverlay = overlayExport
	m.exportInput.Focus()
	return m, nil
}

func (m Model) handleExportKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.activeOverlay = overlayNone
		m.exportInput.Blur()
		m.restoreBrowseStatus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		path := strings.TrimSpace(m.exportInput.Value())
		if path == "" {
			m.status = "provide a " + string(m.exportFormat) + " export path"
			return m, nil
		}
		request, err := m.exportRequest(path)
		if err != nil {
			return m.fail(string(m.exportFormat)+" export failed", err)
		}
		m.activeOverlay = overlayNone
		m.exportInput.Blur()
		m.exportID++
		m.status = "exporting " + string(m.exportFormat) + "..."
		return m, writeExportCmd(m.client, m.timeout, m.exportID, m.exportFormat, request)
	}
	var cmd tea.Cmd
	m.exportInput, cmd = m.exportInput.Update(msg)
	return m, cmd
}

func (m Model) exportRequest(path string) (exportRequest, error) {
	request := exportRequest{
		path:           path,
		columns:        append([]db.Column(nil), m.columns...),
		visibleColumns: append([]bool(nil), m.visibleColumns...),
		filters:        append([]db.RowFilter(nil), m.browseFilters...),
	}
	if m.queryActive {
		if m.queryStreaming {
			request.query = &db.Query{SQL: m.querySQL}
			return request, nil
		}
		request.queryRows = append([][]string(nil), m.queryRows...)
		return request, nil
	}
	table := m.currentTable()
	if table == nil {
		return exportRequest{}, fmt.Errorf("no table selected")
	}
	copy := *table
	request.table = &copy
	return request, nil
}

func (m Model) csvExportRequest(path string) (csvExportRequest, error) { return m.exportRequest(path) }

func (m Model) defaultCSVExportPath(now time.Time) string {
	return m.defaultExportPath(now, exportCSV)
}

func (m Model) defaultJSONExportPath(now time.Time) string {
	return m.defaultExportPath(now, exportJSON)
}

func (m Model) defaultExportPath(now time.Time, format exportFormat) string {
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
	return filepath.Join(directory, name+"-"+now.Format("20060102-150405")+"."+strings.ToLower(string(format)))
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

func writeExportCmd(client db.Driver, timeout time.Duration, exportID uint64, format exportFormat, request exportRequest) tea.Cmd {
	return func() tea.Msg {
		var (
			rows int
			err  error
		)
		switch format {
		case exportCSV:
			rows, err = writeCSV(client, timeout, request)
		case exportJSON:
			rows, err = writeJSON(client, timeout, request)
		default:
			err = fmt.Errorf("unsupported export format %q", format)
		}
		return exportedMsg{exportID: exportID, format: format, path: request.path, rows: rows, err: err}
	}
}

func writeCSV(client db.Driver, timeout time.Duration, request csvExportRequest) (rows int, returnErr error) {
	file, err := createExportFile(request.path, "CSV")
	if err != nil {
		return 0, err
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
	if err := writeExportRows(client, timeout, request, writeRows); err != nil {
		return 0, fmt.Errorf("write CSV rows: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return 0, fmt.Errorf("write CSV: %w", err)
	}
	return rows, nil
}

func writeJSON(client db.Driver, timeout time.Duration, request exportRequest) (rows int, returnErr error) {
	file, err := createExportFile(request.path, "JSON")
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := file.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close JSON file: %w", err)
		}
	}()

	if _, err := file.WriteString("["); err != nil {
		return 0, fmt.Errorf("write JSON opening bracket: %w", err)
	}
	writeRows := func(values [][]string) error {
		for _, row := range values {
			if rows > 0 {
				if _, err := file.WriteString(","); err != nil {
					return err
				}
			}
			record, err := json.Marshal(visibleJSONRow(request.columns, row, request.visibleColumns))
			if err != nil {
				return err
			}
			if _, err := file.Write(record); err != nil {
				return err
			}
			rows++
		}
		return nil
	}
	if err := writeExportRows(client, timeout, request, writeRows); err != nil {
		return 0, fmt.Errorf("write JSON rows: %w", err)
	}
	if _, err := file.WriteString("]\n"); err != nil {
		return 0, fmt.Errorf("write JSON: %w", err)
	}
	return rows, nil
}

func createExportFile(path, format string) (*os.File, error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create export directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s file: %w", format, err)
	}
	return file, nil
}

func writeExportRows(client db.Driver, timeout time.Duration, request exportRequest, writeRows func([][]string) error) error {
	if request.query != nil {
		streamer, ok := client.(db.QueryRowStreamer)
		if !ok {
			return fmt.Errorf("query streaming is unavailable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		stream, err := streamer.OpenQueryRowStream(ctx, *request.query)
		if err != nil {
			return fmt.Errorf("open query stream: %w", err)
		}
		defer stream.Close()
		for {
			values, exhausted, err := db.ReadRowStream(stream, exportBatchSize)
			if err != nil {
				return fmt.Errorf("load query rows: %w", err)
			}
			if err := writeRows(values); err != nil {
				return err
			}
			if exhausted {
				return nil
			}
		}
	}
	if request.table == nil {
		return writeRows(request.queryRows)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for offset := 0; ; offset += exportBatchSize {
		values, err := exportTableRows(ctx, client, *request.table, request.filters, offset)
		if err != nil {
			return fmt.Errorf("load rows: %w", err)
		}
		if err := writeRows(values); err != nil {
			return err
		}
		if len(values) < exportBatchSize {
			return nil
		}
	}
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

func visibleJSONRow(columns []db.Column, row []string, visible []bool) map[string]string {
	values := make(map[string]string, len(columns))
	for i, column := range columns {
		if (len(visible) > 0 && (i >= len(visible) || !visible[i])) || i >= len(row) {
			continue
		}
		values[column.Name] = row[i]
	}
	return values
}
