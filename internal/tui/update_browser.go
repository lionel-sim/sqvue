package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSQLKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.activeOverlay = overlayNone
		m.sqlInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Confirm) {
		sql := strings.TrimSpace(m.sqlInput.Value())
		if sql == "" {
			return m, nil
		}
		m.activeOverlay, m.loading, m.status = overlayNone, true, "running query..."
		m.sqlInput.Blur()
		return m, runQueryCmd(m.client, sql, m.timeout, m.nextRequestID())
	}
	var cmd tea.Cmd
	m.sqlInput, cmd = m.sqlInput.Update(msg)
	return m, cmd
}
func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" || key.Matches(msg, m.keys.Confirm) {
		if msg.String() == "esc" {
			m.filterInput.SetValue(m.filterPrevious)
		}
		m.activeOverlay = overlayNone
		m.filterInput.Blur()
		m.applyFilter()
		return m.startLoad()
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	return m, cmd
}
func (m Model) handleSchemaKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case msg.String() == "esc":
		m.activeOverlay = overlayNone
		m.status = fmt.Sprintf("schema %s", m.currentSchema())
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.schemaCursor = min(m.schemaCursor+1, len(m.schemas)-1)
		m.schemaScroll = keepInView(m.schemaCursor, m.schemaScroll, tableListHeight, len(m.schemas))
	case key.Matches(msg, m.keys.Up):
		m.schemaCursor = max(0, m.schemaCursor-1)
		m.schemaScroll = keepInView(m.schemaCursor, m.schemaScroll, tableListHeight, len(m.schemas))
	case key.Matches(msg, m.keys.Confirm):
		m.schema, m.activeOverlay = m.schemaCursor, overlayNone
		m.selected, m.scroll, m.page = 0, 0, 0
		m.filterInput.SetValue("")
		m.applyFilter()
		m.loading, m.status = true, fmt.Sprintf("loading %s tables...", m.currentSchema())
		return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout, m.nextRequestID())
	}
	return m, nil
}
func (m Model) showDescriptions() (Model, tea.Cmd) {
	if m.mode != modeDescriptions {
		m.queryActive, m.mode = false, modeDescriptions
		return m.startLoadDescriptions()
	}
	return m, nil
}
func (m Model) showValues() (Model, tea.Cmd) {
	if m.mode != modeValues || m.queryActive {
		m.queryActive, m.mode, m.page = false, modeValues, 0
		return m.startLoadRows()
	}
	return m, nil
}
func (m Model) moveSelection(delta int) (Model, tea.Cmd) {
	target := m.selected + delta
	if target < 0 || target >= len(m.tables) {
		return m, nil
	}
	m.selected = target
	m.scroll = keepInView(m.selected, m.scroll, tableListHeight, len(m.tables))
	m.resetBrowseContext()
	m.cellCursor = 0
	m.browseFilters = nil
	return m.startLoad()
}
func keepInView(selected, offset, height, count int) int {
	if count <= height {
		return 0
	}
	if selected < offset {
		return selected
	}
	if selected >= offset+height {
		return selected - height + 1
	}
	return offset
}
func (m Model) changePage(delta int) (Model, tea.Cmd) {
	if m.mode != modeValues || m.page+delta < 0 {
		return m, nil
	}
	if m.queryActive {
		if delta > 0 && (m.page+1)*m.pageSize >= len(m.queryRows) {
			return m, nil
		}
		m.page += delta
		m.setQueryPage()
		return m, nil
	}
	if delta > 0 && !m.hasNextPage {
		return m, nil
	}
	m.page += delta
	return m.startLoadRows()
}
func (m Model) handleTablesLoaded(msg tablesLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		return m.fail("failed to load tables", msg.err)
	}
	m.allTables, m.lastErr = msg.tables, nil
	m.rowCounts, m.browseRowCounts = make(map[string]int64), make(map[string]int64)
	m.applyFilter()
	m.status = fmt.Sprintf("found %d tables", len(m.tables))
	if len(m.tables) > 0 {
		m.selected, m.scroll, m.page, m.rowCursor, m.cellCursor, m.mode = 0, 0, 0, 0, 0, modeValues
		return m.startLoadRows()
	}
	return m, nil
}
func (m Model) handleSchemasLoaded(msg schemasLoadedMsg) (Model, tea.Cmd) {
	if !m.isCurrent(msg.requestID) {
		return m, nil
	}
	m.loading = false
	if msg.err != nil {
		return m.fail("failed to load schemas", msg.err)
	}
	m.schemas, m.lastErr = msg.schemas, nil
	if len(m.schemas) == 0 {
		return m.fail("failed to load schemas", fmt.Errorf("no user schemas found"))
	}
	for i, schema := range m.schemas {
		if schema.Name == "public" {
			m.schema = i
			break
		}
	}
	m.loading, m.status = true, fmt.Sprintf("loading %s tables...", m.currentSchema())
	return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout, m.nextRequestID())
}
func (m *Model) applyFilter() {
	needle := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.tables = m.tables[:0]
	for _, table := range m.allTables {
		if needle == "" || strings.Contains(strings.ToLower(table.Name), needle) {
			m.tables = append(m.tables, table)
		}
	}
	m.selected, m.scroll = 0, 0
	m.resetBrowseContext()
}
func (m Model) currentSchema() string {
	if m.schema < 0 || m.schema >= len(m.schemas) {
		return ""
	}
	return m.schemas[m.schema].Name
}
