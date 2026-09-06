package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m, cmd, handled := m.handleOverlayKey(msg); handled {
		return m, cmd
	}
	switch {
	case key.Matches(msg, m.keys.Help):
		m.activeOverlay = overlayHelp
	case msg.String() == "esc" && m.focused:
		m.focused = false
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Confirm):
		if m.focused {
			m.activeOverlay, m.detailScroll = overlayRowDetail, 0
		} else if m.mode == modeValues && len(m.rows) > 0 {
			m.focused = true
		}
	case key.Matches(msg, m.keys.ExportCSV) && !m.focused:
		return m.beginCSVExport()
	case key.Matches(msg, m.keys.ExportJSON):
		return m.beginJSONExport()
	case key.Matches(msg, m.keys.Backup):
		return m.beginBackup()
	case m.focused:
		return m.handleGridKey(msg)
	default:
		return m.handleBrowserKey(msg)
	}
	return m, nil
}

func (m Model) handleBrowserKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Refresh):
		m.loading, m.status = true, "reloading tables..."
		requestID := m.nextRequestID()
		return m, loadTablesCmd(m.client, m.currentSchema(), m.timeout, requestID)
	case key.Matches(msg, m.keys.Schema):
		if len(m.schemas) > 0 {
			m.activeOverlay, m.schemaCursor = overlaySchemaPicker, m.schema
			m.schemaScroll = keepInView(m.schemaCursor, m.schemaScroll, tableListHeight, len(m.schemas))
			m.status = "select a schema"
		}
	case key.Matches(msg, m.keys.Filter):
		m.filterPrevious, m.activeOverlay = m.filterInput.Value(), overlayFilter
		m.filterInput.Focus()
	case key.Matches(msg, m.keys.SQL):
		m.activeOverlay = overlaySQL
		m.sqlInput.Focus()
	case key.Matches(msg, m.keys.Columns):
		if m.mode == modeValues && len(m.columns) > 0 {
			m.activeOverlay, m.columnCursor, m.columnScroll = overlayColumnPicker, 0, 0
			m.status = "choose visible columns"
		}
	case key.Matches(msg, m.keys.Down):
		return m.moveSelection(+1)
	case key.Matches(msg, m.keys.Up):
		return m.moveSelection(-1)
	case key.Matches(msg, m.keys.PageDown):
		return m.changePage(+1)
	case key.Matches(msg, m.keys.PageUp):
		return m.changePage(-1)
	case key.Matches(msg, m.keys.ShowDescriptions):
		return m.showDescriptions()
	case key.Matches(msg, m.keys.ShowValues):
		return m.showValues()
	}
	return m, nil
}
