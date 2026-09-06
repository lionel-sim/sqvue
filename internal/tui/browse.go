package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

const sortStatusSuffix = " sort: "

func (m Model) handleRowsLoaded(msg rowsLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.cellEditRefreshPending = false
		return m.fail("rows error", msg.err)
	}
	if msg.columns != nil {
		m.columns = msg.columns
	}
	if t := m.currentTable(); t != nil {
		m.ensureVisibleColumns(t.String())
	}
	m.rows, m.hasNextPage = msg.rows, len(msg.rows) > m.pageSize
	if m.hasNextPage {
		m.rows = m.rows[:m.pageSize]
	}
	if m.rowCursor >= len(m.rows) {
		m.rowCursor = max(0, len(m.rows)-1)
	}
	if m.pendingRowMoves != 0 {
		pending := m.pendingRowMoves
		m.pendingRowMoves = 0
		switch {
		case pending >= 0 && pending < len(m.rows):
			m.rowCursor = pending
		case pending >= len(m.rows) && m.hasNextPage:
			m.page++
			m.pendingRowMoves = pending - len(m.rows)
			return m.startLoadRows()
		case pending < 0 && m.page > 0:
			m.page--
			m.pendingRowMoves = pending + len(m.rows)
			return m.startLoadRows()
		case pending < 0:
			m.rowCursor = 0
		default:
			m.rowCursor = max(0, len(m.rows)-1)
		}
	}
	m.cellCursor = clamp(m.cellCursor, 0, max(0, m.displayedColumnCount()-1))
	m.lastErr = nil
	if m.cellEditRefreshPending {
		m.cellEditRefreshPending = false
		m.status = "updated " + sanitizeText(m.cellEditColumn)
		return m, nil
	}
	if t := m.currentTable(); t != nil {
		m.setRowsStatus(*t)
		return m, m.loadCurrentRowCount(*t)
	}
	return m, nil
}

func (m Model) handleTableStreamRowsLoaded(msg tableStreamRowsLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m.discardTableStreamMessage(msg)
	}
	m.loading = false
	if msg.err != nil {
		return m.failTableStreamMessage(msg)
	}
	m.applyTableStreamMetadata(msg)
	if handled, model, cmd := m.handleEmptyTableStreamPage(msg); handled {
		return model, cmd
	}
	m.applyTableStreamRows(msg)
	var cmd tea.Cmd
	var loading bool
	m, cmd, loading = m.applyPendingRowMove(Model.startLoadRows)
	if loading {
		return m, cmd
	}
	if msg.exhausted {
		m.closeTableStream()
	}
	m.cellCursor = clamp(m.cellCursor, 0, max(0, m.displayedColumnCount()-1))
	m.lastErr = nil
	if m.cellEditRefreshPending {
		m.cellEditRefreshPending = false
		m.status = "updated " + sanitizeText(m.cellEditColumn)
		return m, nil
	}
	if t := m.currentTable(); t != nil {
		m.setRowsStatus(*t)
		if m.tableStream.stream == nil {
			return m, m.loadCurrentRowCount(*t)
		}
	}
	return m, nil
}

func (m Model) discardTableStreamMessage(msg tableStreamRowsLoadedMsg) (Model, tea.Cmd) {
	if msg.stream != nil {
		_ = msg.stream.Close()
	}
	if msg.cancel != nil {
		msg.cancel()
	}
	return m, nil
}
func (m Model) failTableStreamMessage(msg tableStreamRowsLoadedMsg) (Model, tea.Cmd) {
	m.cellEditRefreshPending = false
	if msg.stream != nil {
		_ = msg.stream.Close()
	}
	if msg.cancel != nil {
		msg.cancel()
	}
	m.closeTableStream()
	return m.fail("rows error", msg.err)
}
func (m *Model) applyTableStreamMetadata(msg tableStreamRowsLoadedMsg) {
	if msg.stream != nil {
		m.tableStream = tableStreamState{stream: msg.stream, cancel: msg.cancel, key: m.browseStreamKeyForCurrentTable(), nextPage: m.page + 1}
	}
	if msg.rowCount != nil {
		if msg.browseKey != "" {
			m.browseRowCounts[msg.browseKey] = *msg.rowCount
		} else if t := m.currentTable(); t != nil {
			m.rowCounts[t.String()] = *msg.rowCount
		}
	}
}
func (m Model) handleEmptyTableStreamPage(msg tableStreamRowsLoadedMsg) (bool, Model, tea.Cmd) {
	if len(msg.rows) != 0 || !msg.exhausted || m.page == 0 {
		return false, m, nil
	}
	m.page--
	m.hasNextPage = false
	m.closeTableStream()
	if t := m.currentTable(); t != nil {
		m.setRowsStatus(*t)
		return true, m, m.loadCurrentRowCount(*t)
	}
	return true, m, nil
}
func (m *Model) applyTableStreamRows(msg tableStreamRowsLoadedMsg) {
	if msg.columns != nil {
		m.columns = msg.columns
	}
	if t := m.currentTable(); t != nil {
		m.ensureVisibleColumns(t.String())
	}
	m.rows, m.tableStream.pending, m.hasNextPage = splitStreamPage(msg.rows, m.pageSize, msg.exhausted)
	if m.rowCursor >= len(m.rows) {
		m.rowCursor = max(0, len(m.rows)-1)
	}
	if m.tableStream.stream != nil {
		m.tableStream.nextPage = m.page + 1
	}
}
func (m Model) applyPendingRowMove(load func(Model) (Model, tea.Cmd)) (Model, tea.Cmd, bool) {
	if m.pendingRowMoves == 0 {
		return m, nil, false
	}
	pending := m.pendingRowMoves
	m.pendingRowMoves = 0
	switch {
	case pending >= 0 && pending < len(m.rows):
		m.rowCursor = pending
	case pending >= len(m.rows) && m.hasNextPage:
		m.page++
		m.pendingRowMoves = pending - len(m.rows)
		next, cmd := load(m)
		return next, cmd, true
	case pending < 0 && m.page > 0:
		m.page--
		m.pendingRowMoves = pending + len(m.rows)
		next, cmd := load(m)
		return next, cmd, true
	case pending < 0:
		m.rowCursor = 0
	default:
		m.rowCursor = max(0, len(m.rows)-1)
	}
	return m, nil, false
}
func (m Model) handleCountLoaded(msg countLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) || msg.err != nil {
		return m, nil
	}
	if msg.browseKey != "" {
		m.browseRowCounts[msg.browseKey] = msg.count
	} else {
		m.rowCounts[msg.table.String()] = msg.count
	}
	if t := m.currentTable(); t != nil && *t == msg.table && !m.queryActive {
		m.setRowsStatus(*t)
	}
	return m, nil
}
func (m Model) handleQueryLoaded(msg queryLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		m.activeOverlay = overlaySQL
		m.sqlInput.Focus()
		m.status, m.lastErr = sqlErrorStatus(m.querySourceSQL, msg.err), msg.err
		return m, nil
	}
	m.closeQueryStream()
	m.queryActive, m.queryStreaming, m.lastErr, m.mode, m.page, m.rowCursor, m.cellCursor = true, false, nil, modeValues, 0, 0, 0
	if !m.queryPreserveEditor {
		m.querySort = db.SortSpec{}
	}
	m.querySQL, m.querySourceSQL = msg.sql, msg.sql
	m.queryDuration, m.queryAffected, m.queryTruncated = msg.result.DurationMs, msg.result.RowsAffected, msg.result.Truncated
	previousColumns, previousVisible := m.columns, m.visibleColumns
	m.columns = make([]db.Column, len(msg.result.Columns))
	for i, name := range msg.result.Columns {
		m.columns[i] = db.Column{Name: name}
	}
	m.visibleColumns, m.visibleColumnKey = refreshedQueryColumns(previousColumns, previousVisible, m.columns, m.queryPreserveEditor)
	m.queryRows = make([][]string, len(msg.result.Rows))
	for i, row := range msg.result.Rows {
		m.queryRows[i] = make([]string, len(row))
		for j, value := range row {
			m.queryRows[i][j] = formatQueryValue(value)
		}
	}
	m.queryBaseRows = append([][]string(nil), m.queryRows...)
	if m.queryPreserveEditor {
		sortQueryRows(m.queryRows, m.columns, m.querySort)
	}
	m.setQueryPage()
	if !m.queryPreserveEditor {
		m.sqlInput.SetValue("")
	}
	m.queryPreserveEditor = false
	return m, nil
}

func refreshedQueryColumns(previous []db.Column, visible []bool, current []db.Column, preserve bool) ([]bool, string) {
	if preserve && len(previous) == len(current) && len(visible) == len(current) {
		matches := true
		for i := range current {
			if previous[i].Name != current[i].Name {
				matches = false
				break
			}
		}
		if matches {
			return append([]bool(nil), visible...), "query"
		}
	}
	visible = make([]bool, len(current))
	for i := range visible {
		visible[i] = true
	}
	return visible, "query"
}

func (m Model) handleQueryStreamRowsLoaded(msg queryStreamRowsLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m.discardQueryStreamMessage(msg)
	}
	m.loading = false
	if msg.err != nil {
		return m.failQueryStreamMessage(msg)
	}
	if msg.initial {
		previousColumns, previousVisible := m.columns, m.visibleColumns
		m.queryActive, m.queryStreaming, m.lastErr, m.mode, m.page, m.rowCursor, m.cellCursor = true, true, nil, modeValues, 0, 0, 0
		m.querySQL, m.queryRows, m.queryBaseRows, m.queryTruncated = msg.sql, nil, nil, false
		m.columns = make([]db.Column, len(msg.columns))
		for i, name := range msg.columns {
			m.columns[i] = db.Column{Name: name}
		}
		m.visibleColumns, m.visibleColumnKey = refreshedQueryColumns(previousColumns, previousVisible, m.columns, m.queryPreserveEditor)
	}
	if msg.stream != nil {
		m.queryStream, m.queryStreamCancel, m.queryStreamNextPage = msg.stream, msg.cancel, m.page+1
	}
	if len(msg.rows) == 0 && msg.exhausted && m.page > 0 {
		m.page--
		m.hasNextPage = false
		m.closeQueryStream()
		m.setQueryPage()
		return m, nil
	}
	m.rows, m.queryStreamPending, m.hasNextPage = splitStreamPage(msg.rows, m.pageSize, msg.exhausted)
	m.queryDuration, m.queryAffected = msg.duration, msg.affected
	if m.rowCursor >= len(m.rows) {
		m.rowCursor = max(0, len(m.rows)-1)
	}
	if m.queryStream != nil {
		m.queryStreamNextPage = m.page + 1
	}
	var cmd tea.Cmd
	var loading bool
	m, cmd, loading = m.applyPendingRowMove(Model.startLoadQueryRows)
	if loading {
		return m, cmd
	}
	if msg.exhausted {
		m.closeQueryStream()
	}
	m.cellCursor = clamp(m.cellCursor, 0, max(0, m.displayedColumnCount()-1))
	m.lastErr = nil
	m.setQueryPage()
	if !m.queryPreserveEditor {
		m.sqlInput.SetValue("")
	}
	m.queryPreserveEditor = false
	return m, nil
}
func (m Model) discardQueryStreamMessage(msg queryStreamRowsLoadedMsg) (Model, tea.Cmd) {
	if msg.stream != nil {
		_ = msg.stream.Close()
	}
	if msg.cancel != nil {
		msg.cancel()
	}
	return m, nil
}
func (m Model) failQueryStreamMessage(msg queryStreamRowsLoadedMsg) (Model, tea.Cmd) {
	if msg.stream != nil {
		_ = msg.stream.Close()
	}
	if msg.cancel != nil {
		msg.cancel()
	}
	m.closeQueryStream()
	m.activeOverlay = overlaySQL
	m.sqlInput.Focus()
	m.status, m.lastErr = sqlErrorStatus(m.querySourceSQL, msg.err), msg.err
	return m, nil
}
func formatQueryValue(value any) string {
	if value == nil {
		return "NULL"
	}
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return fmt.Sprint(value)
}
func (m *Model) setQueryPage() {
	if m.queryStreaming {
		m.status = fmt.Sprintf("query page %d (%d rows, %d ms, %d affected)", m.page+1, len(m.rows), m.queryDuration, m.queryAffected)
		if sort := formatSort(m.querySort); sort != "" {
			m.status += sortStatusSuffix + sort
		}
		return
	}
	start := min(m.page*m.pageSize, len(m.queryRows))
	end := min(start+m.pageSize, len(m.queryRows))
	m.rows = m.queryRows[start:end]
	m.status = fmt.Sprintf("query page %d (%d/%d rows, %d ms, %d affected)", m.page+1, len(m.rows), len(m.queryRows), m.queryDuration, m.queryAffected)
	if m.queryTruncated {
		m.status += " [limited to 1000 rows]"
	}
	if sort := formatSort(m.querySort); sort != "" {
		m.status += sortStatusSuffix + sort
	}
}
func (m *Model) setRowsStatus(t db.Table) {
	status := fmt.Sprintf("%s page %d (%d rows", t.String(), m.page+1, len(m.rows))
	if count, ok := m.browseRowCounts[m.browseCountKey(t)]; len(m.browseFilters) > 0 && ok {
		status += fmt.Sprintf(" of %d", count)
	} else if len(m.browseFilters) == 0 {
		if count, ok := m.rowCounts[t.String()]; ok {
			status += fmt.Sprintf(" of %d", count)
		}
	}
	status += ")"
	if len(m.browseFilters) > 0 {
		status += " filters: " + formatBrowseFilters(m.browseFilters) + " (x clear)"
	}
	if sort := formatSort(m.browseSort); sort != "" {
		status += sortStatusSuffix + sort
	}
	m.status = status
}

func (m *Model) restoreBrowseStatus() {
	if m.queryActive {
		m.setQueryPage()
		return
	}
	if table := m.currentTable(); table != nil {
		m.setRowsStatus(*table)
	}
}
func formatBrowseFilters(filters []db.RowFilter) string {
	parts := make([]string, len(filters))
	for i, filter := range filters {
		parts[i] = filter.Column + " " + browseFilterOperatorLabel(filter.Operator)
		if filter.Operator != db.FilterIsNull && filter.Operator != db.FilterIsNotNull {
			parts[i] += " " + filter.Value
		}
	}
	return strings.Join(parts, " AND ")
}
func (m Model) browseRequest(t db.Table) db.BrowseRequest {
	return db.BrowseRequest{Table: t, Filters: append([]db.RowFilter(nil), m.browseFilters...), Sort: m.browseSort}
}
func (m Model) browseStreamKey(t db.Table) string {
	return fmt.Sprintf("%s\x00sort\x00%s\x00%t", m.browseCountKey(t), m.browseSort.Column, m.browseSort.Descending)
}
func (m Model) browseStreamKeyForCurrentTable() string {
	if t := m.currentTable(); t != nil {
		return m.browseStreamKey(*t)
	}
	return ""
}

func (m *Model) closeTableStream() {
	if m.tableStream.cancel != nil {
		m.tableStream.cancel()
	}
	if m.tableStream.stream != nil {
		_ = m.tableStream.stream.Close()
	}
	m.tableStream = tableStreamState{}
}

func (m *Model) closeQueryStream() {
	if m.queryStreamCancel != nil {
		m.queryStreamCancel()
	}
	if m.queryStream != nil {
		_ = m.queryStream.Close()
	}
	m.queryStream, m.queryStreamCancel, m.queryStreamNextPage, m.queryStreamPending = nil, nil, 0, nil
}

func (m Model) startLoadQueryRows() (Model, tea.Cmd) {
	streamer, ok := m.client.(db.QueryRowStreamer)
	if !ok || !m.queryStreaming {
		return m, nil
	}
	m.loading = true
	m.status = fmt.Sprintf("loading query page %d...", m.page+1)
	requestID := m.nextRequestID()
	if m.queryStream != nil && m.page == m.queryStreamNextPage {
		return m, readQueryRowStreamCmd(m.queryStream, m.pageSize, m.queryStreamPending, requestID)
	}
	m.closeQueryStream()
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	m.queryStreamCancel = cancel
	return m, openQueryRowStreamCmd(queryStreamLoadRequest{ctx: ctx, cancel: cancel, streamer: streamer, query: db.Query{SQL: m.querySQL}, pageSize: m.pageSize, skip: m.page * m.pageSize, requestID: requestID})
}

func splitStreamPage(rows [][]string, pageSize int, exhausted bool) ([][]string, [][]string, bool) {
	if len(rows) <= pageSize {
		return rows, nil, !exhausted
	}
	return rows[:pageSize], rows[pageSize:], true
}

func (m Model) loadCurrentRowCount(t db.Table) tea.Cmd {
	if len(m.browseFilters) > 0 {
		if _, ok := m.browseRowCounts[m.browseCountKey(t)]; !ok {
			return loadBrowseCountCmd(m.client, m.browseRequest(t), m.timeout, m.loadID)
		}
		return nil
	}
	if _, ok := m.rowCounts[t.String()]; !ok {
		return loadCountCmd(m.client, t, m.timeout, m.loadID)
	}
	return nil
}
func (m Model) browseCountKey(t db.Table) string {
	var b strings.Builder
	b.WriteString(t.String())
	for _, filter := range m.browseFilters {
		fmt.Fprintf(&b, "\x00%s\x00%s\x00%s", filter.Column, filter.Operator, filter.Value)
	}
	return b.String()
}
func (m Model) handleDescriptionsLoaded(msg descriptionsLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		return m.fail("describe failed", msg.err)
	}
	m.tableInfo, m.lastErr = msg.info, nil
	if t := m.currentTable(); t != nil {
		m.status = fmt.Sprintf("%s (%d columns)", t.String(), len(msg.info.Columns))
	}
	return m, nil
}
