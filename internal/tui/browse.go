package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func (m Model) handleRowsLoaded(msg rowsLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		return m.fail("rows error", msg.err)
	}
	m.columns = msg.columns
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
	if t := m.currentTable(); t != nil {
		m.setRowsStatus(*t)
		if len(m.browseFilters) > 0 {
			if _, ok := m.browseRowCounts[m.browseCountKey(*t)]; !ok {
				return m, loadBrowseCountCmd(m.client, m.browseRequest(*t), m.timeout, m.loadID)
			}
		} else if _, ok := m.rowCounts[t.String()]; !ok {
			return m, loadCountCmd(m.client, *t, m.timeout, m.loadID)
		}
	}
	return m, nil
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
		return m.fail("query failed", msg.err)
	}
	m.queryActive, m.lastErr, m.mode, m.page, m.rowCursor, m.cellCursor = true, nil, modeValues, 0, 0, 0
	m.queryDuration, m.queryAffected, m.queryTruncated = msg.result.DurationMs, msg.result.RowsAffected, msg.result.Truncated
	m.columns = make([]db.Column, len(msg.result.Columns))
	for i, name := range msg.result.Columns {
		m.columns[i] = db.Column{Name: name}
	}
	m.visibleColumns, m.visibleColumnKey = make([]bool, len(m.columns)), "query"
	for i := range m.visibleColumns {
		m.visibleColumns[i] = true
	}
	m.queryRows = make([][]string, len(msg.result.Rows))
	for i, row := range msg.result.Rows {
		m.queryRows[i] = make([]string, len(row))
		for j, value := range row {
			m.queryRows[i][j] = formatQueryValue(value)
		}
	}
	m.setQueryPage()
	m.sqlInput.SetValue("")
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
	start := min(m.page*m.pageSize, len(m.queryRows))
	end := min(start+m.pageSize, len(m.queryRows))
	m.rows = m.queryRows[start:end]
	m.status = fmt.Sprintf("query page %d (%d/%d rows, %d ms, %d affected)", m.page+1, len(m.rows), len(m.queryRows), m.queryDuration, m.queryAffected)
	if m.queryTruncated {
		m.status += " [limited to 1000 rows]"
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
	m.status = status
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
	return db.BrowseRequest{Table: t, Filters: append([]db.RowFilter(nil), m.browseFilters...)}
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
