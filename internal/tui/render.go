package tui

import (
	"strings"
)

const (
	maxColWidth     = 40
	minColWidth     = 2
	colSep          = " | "
	tableListHeader = 1
	tableListHeight = 4
	gapAfterList    = 1
	columnsHeader   = 1
	maxPageSize     = 50
	footerRows      = 1
	// reservedRows counts every fixed line above and below the data rows:
	// list header, list, blank gap, column header, and the footer.
	reservedRows = tableListHeader + tableListHeight + gapAfterList + columnsHeader + footerRows
)

// Render composes the active sqvue screen.
func Render(m Model) string {
	switch m.activeOverlay {
	case overlayHelp:
		return renderHelpModal(m)
	case overlayRowDetail:
		return renderRowDetailModal(m)
	}
	return renderMain(m)
}

func renderMain(m Model) string {
	var b strings.Builder

	if m.activeOverlay == overlaySchemaPicker {
		renderSchemaList(&b, m.schemas, m.schemaCursor, m.schemaScroll)
	} else {
		renderTableList(&b, m.currentSchema(), m.tables, m.selected, m.scroll, m.filterInput.Value())
	}
	b.WriteString("\n")
	if m.activeOverlay == overlayColumnPicker {
		renderColumnPicker(&b, m.columns, m.visibleColumns, m.columnCursor, m.columnScroll, m.columnPickerHeight())
	} else if m.activeOverlay == overlaySQL {
		b.WriteString(m.sqlInput.View() + "\n")
	} else if m.activeOverlay == overlayFilter {
		b.WriteString(m.filterInput.View() + "\n")
	} else if m.activeOverlay == overlayBrowseFilter {
		b.WriteString(m.browseFilterInput.View() + "  (Tab changes operator)\n")
	} else if m.activeOverlay == overlaySchemaPicker {
		b.WriteString("Use j/k to choose a schema, then Enter to load its tables.\n")
	} else if m.loading {
		b.WriteString("Loading...\n")
	} else if m.mode == modeDescriptions {
		renderDescriptions(&b, m.tableInfo.Columns, m.width, m.height-reservedRows)
	} else {
		activeRow := -1
		if m.focused {
			activeRow = m.rowCursor
		}
		renderVisibleRows(&b, m.columns, m.rows, m.visibleColumns, m.width, m.height-reservedRows, activeRow, m.cellCursor)
	}
	padToFooter(&b, m.height)
	b.WriteString(renderFooter(m))

	return b.String()
}
