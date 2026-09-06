package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func loadSchemasCmd(c db.Driver, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		schemas, err := c.ListSchemas(ctx)
		return schemasLoadedMsg{requestID: requestID, schemas: schemas, err: err}
	}
}

func loadTablesCmd(c db.Driver, schema string, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		tables, err := c.ListTables(ctx, schema)
		return tablesLoadedMsg{requestID: requestID, tables: tables, err: err}
	}
}

func loadRowsCmd(c db.Driver, tbl db.Table, limit, offset int, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cols, rows, err := c.Rows(ctx, tbl, limit, offset)
		return rowsLoadedMsg{requestID: requestID, columns: cols, rows: rows, err: err}
	}
}

func loadBrowseRowsCmd(c db.Driver, req db.BrowseRequest, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cols, rows, err := c.BrowseRows(ctx, req)
		return rowsLoadedMsg{requestID: requestID, columns: cols, rows: rows, err: err}
	}
}

type tableStreamLoadRequest struct {
	ctx       context.Context
	cancel    func()
	client    db.Driver
	streamer  db.TableRowStreamer
	request   db.TableRowStreamRequest
	loadCount bool
	pageSize  int
	skip      int
	requestID uint64
}

func openTableRowStreamCmd(load tableStreamLoadRequest) tea.Cmd {
	return func() tea.Msg {
		var rowCount *int64
		browseKey := ""
		if load.loadCount {
			var (
				count int64
				err   error
			)
			if len(load.request.Filters) > 0 {
				browse := db.BrowseRequest{Table: load.request.Table, Filters: load.request.Filters, Sort: load.request.Sort}
				count, err = load.client.CountBrowseRows(load.ctx, browse)
				browseKey = browseCountKey(browse)
			} else {
				count, err = load.client.CountRows(load.ctx, load.request.Table)
			}
			if err == nil {
				rowCount = &count
			}
		}
		columns, stream, err := load.streamer.OpenTableRowStream(load.ctx, load.request)
		if err != nil {
			return tableStreamRowsLoadedMsg{requestID: load.requestID, rowCount: rowCount, browseKey: browseKey, cancel: load.cancel, err: err}
		}
		if load.skip > 0 {
			if _, _, err := db.ReadRowStream(stream, load.skip); err != nil {
				_ = stream.Close()
				return tableStreamRowsLoadedMsg{requestID: load.requestID, rowCount: rowCount, browseKey: browseKey, cancel: load.cancel, err: err}
			}
		}
		rows, exhausted, err := db.ReadRowStream(stream, load.pageSize+1)
		if err != nil {
			_ = stream.Close()
			return tableStreamRowsLoadedMsg{requestID: load.requestID, rowCount: rowCount, browseKey: browseKey, cancel: load.cancel, err: err}
		}
		return tableStreamRowsLoadedMsg{requestID: load.requestID, columns: columns, rows: rows, exhausted: exhausted, rowCount: rowCount, browseKey: browseKey, stream: stream, cancel: load.cancel}
	}
}

func readTableRowStreamCmd(stream db.RowStream, pageSize int, pending [][]string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		rows := append([][]string(nil), pending...)
		loaded, exhausted, err := db.ReadRowStream(stream, pageSize+1-len(rows))
		rows = append(rows, loaded...)
		return tableStreamRowsLoadedMsg{requestID: requestID, rows: rows, exhausted: exhausted, err: err}
	}
}

func loadDescriptionsCmd(c db.Driver, tbl db.Table, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		info, err := c.DescribeTable(ctx, tbl.Schema, tbl.Name)
		return descriptionsLoadedMsg{requestID: requestID, info: info, err: err}
	}
}

func loadCountCmd(c db.Driver, tbl db.Table, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		count, err := c.CountRows(ctx, tbl)
		return countLoadedMsg{requestID: requestID, table: tbl, count: count, err: err}
	}
}

func loadBrowseCountCmd(c db.Driver, req db.BrowseRequest, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		count, err := c.CountBrowseRows(ctx, req)
		return countLoadedMsg{requestID: requestID, table: req.Table, count: count, browseKey: browseCountKey(req), err: err}
	}
}

func browseCountKey(req db.BrowseRequest) string {
	key := req.Table.String()
	for _, filter := range req.Filters {
		key += "\x00" + filter.Column + "\x00" + string(filter.Operator) + "\x00" + filter.Value
	}
	return key
}

func runQueryCmd(c db.Driver, sql string, timeout time.Duration, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := c.Query(ctx, db.Query{SQL: sql})
		return queryLoadedMsg{requestID: requestID, sql: sql, result: result, err: err}
	}
}

func updateCellCmd(updater db.CellUpdater, request db.CellUpdateRequest, timeout time.Duration, updateID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := updater.UpdateCell(ctx, request)
		return cellUpdatedMsg{updateID: updateID, column: request.Column, err: err}
	}
}

func insertRowCmd(inserter db.RowInserter, request db.RowInsertRequest, timeout time.Duration, insertID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := inserter.InsertRow(ctx, request)
		return rowInsertedMsg{insertID: insertID, err: err}
	}
}

type queryStreamLoadRequest struct {
	ctx       context.Context
	cancel    func()
	streamer  db.QueryRowStreamer
	query     db.Query
	pageSize  int
	skip      int
	requestID uint64
	initial   bool
}

func openQueryRowStreamCmd(load queryStreamLoadRequest) tea.Cmd {
	return func() tea.Msg {
		stream, err := load.streamer.OpenQueryRowStream(load.ctx, load.query)
		if err != nil {
			return queryStreamRowsLoadedMsg{requestID: load.requestID, sql: load.query.SQL, initial: load.initial, cancel: load.cancel, err: err}
		}
		if load.skip > 0 {
			if _, _, err := db.ReadRowStream(stream, load.skip); err != nil {
				_ = stream.Close()
				return queryStreamRowsLoadedMsg{requestID: load.requestID, sql: load.query.SQL, initial: load.initial, cancel: load.cancel, err: err}
			}
		}
		rows, exhausted, err := db.ReadRowStream(stream, load.pageSize+1)
		if err != nil {
			_ = stream.Close()
			return queryStreamRowsLoadedMsg{requestID: load.requestID, sql: load.query.SQL, initial: load.initial, cancel: load.cancel, err: err}
		}
		return queryStreamRowsLoadedMsg{requestID: load.requestID, sql: load.query.SQL, initial: load.initial, columns: stream.Columns(), rows: rows, exhausted: exhausted, stream: stream, cancel: load.cancel, duration: stream.DurationMs(), affected: stream.RowsAffected()}
	}
}

func readQueryRowStreamCmd(stream db.QueryRowStream, pageSize int, pending [][]string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		rows := append([][]string(nil), pending...)
		loaded, exhausted, err := db.ReadRowStream(stream, pageSize+1-len(rows))
		rows = append(rows, loaded...)
		return queryStreamRowsLoadedMsg{requestID: requestID, rows: rows, exhausted: exhausted, duration: stream.DurationMs(), affected: stream.RowsAffected(), err: err}
	}
}

func writeClipboardCmd(value, kind string, copyStatusID uint64) tea.Cmd {
	return func() tea.Msg {
		return clipboardWrittenMsg{kind: kind, copyStatusID: copyStatusID, err: clipboard.WriteAll(value)}
	}
}

const copyStatusDuration = 2 * time.Second

func clearCopyStatusCmd(kind string, copyStatusID uint64) tea.Cmd {
	return tea.Tick(copyStatusDuration, func(time.Time) tea.Msg {
		return copyStatusClearedMsg{kind: kind, copyStatusID: copyStatusID}
	})
}

// startLoad dispatches to the loader for the current view mode.
func (m Model) startLoad() (Model, tea.Cmd) {
	if m.mode == modeDescriptions {
		m.closeTableStream()
		return m.startLoadDescriptions()
	}
	return m.startLoadRows()
}

func (m Model) startLoadRows() (Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.closeQueryStream()
		m.queryActive = false
		m.loading = true
		m.status = fmt.Sprintf("loading %s page %d...", t.String(), m.page+1)
		requestID := m.nextRequestID()
		if streamer, ok := m.client.(db.TableRowStreamer); ok {
			key := m.browseStreamKey(*t)
			if m.tableStream.stream != nil && m.tableStream.key == key && m.page == m.tableStream.nextPage {
				return m, readTableRowStreamCmd(m.tableStream.stream, m.pageSize, m.tableStream.pending, requestID)
			}
			m.closeTableStream()
			ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
			m.tableStream = tableStreamState{cancel: cancel, key: key}
			loadCount := false
			if len(m.browseFilters) > 0 {
				_, loadCount = m.browseRowCounts[m.browseCountKey(*t)]
				loadCount = !loadCount
			} else {
				_, loadCount = m.rowCounts[t.String()]
				loadCount = !loadCount
			}
			return m, openTableRowStreamCmd(tableStreamLoadRequest{ctx: ctx, cancel: cancel, client: m.client, streamer: streamer, request: db.TableRowStreamRequest{Table: *t, Filters: append([]db.RowFilter(nil), m.browseFilters...), Sort: m.browseSort}, loadCount: loadCount, pageSize: m.pageSize, skip: m.page * m.pageSize, requestID: requestID})
		}
		offset := m.page * m.pageSize
		if len(m.browseFilters) > 0 || m.browseSort.Column != "" {
			req := m.browseRequest(*t)
			req.Limit = m.pageSize + 1
			req.Offset = offset
			return m, loadBrowseRowsCmd(m.client, req, m.timeout, requestID)
		}
		// Fetch one extra row to determine whether a following page exists.
		return m, loadRowsCmd(m.client, *t, m.pageSize+1, offset, m.timeout, requestID)
	}
	return m, nil
}

func (m Model) startLoadDescriptions() (Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.loading = true
		m.status = fmt.Sprintf("loading %s columns...", t.String())
		requestID := m.nextRequestID()
		return m, loadDescriptionsCmd(m.client, *t, m.timeout, requestID)
	}
	return m, nil
}
