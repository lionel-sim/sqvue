package tui

import (
	"strings"

	"sqvue/internal/db"
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
	case overlayBackupScope:
		return renderModalOverMain(m, renderBackupScopeModal(m))
	case overlayBackupPath:
		return renderModalOverMain(m, renderBackupPathModal(m))
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
	} else if m.activeOverlay == overlayExport {
		b.WriteString(m.exportInput.View() + "\n")
	} else if m.activeOverlay == overlayFilter {
		b.WriteString(m.filterInput.View() + "\n")
	} else if m.activeOverlay == overlayBrowseFilterOperator {
		renderBrowseFilterOperatorPicker(&b, m.browseFilterColumn, m.browseFilterCursor, m.browseFilterOperators(), max(1, m.height-reservedRows-1))
	} else if m.activeOverlay == overlayBrowseFilter {
		b.WriteString(m.browseFilterInput.View() + "\n")
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
		renderVisibleRows(&b, m.columns, m.rows, rowRenderOptions{
			visibleColumns: m.visibleColumns,
			width:          m.width,
			maxRows:        m.height - reservedRows,
			activeRow:      activeRow,
			activeColumn:   m.cellCursor,
		})
	}
	padToFooter(&b, m.height)
	b.WriteString(renderFooter(m))

	return b.String()
}

func renderBrowseFilterOperatorPicker(b *strings.Builder, column string, cursor int, operators []db.FilterOperator, height int) {
	b.WriteString("Filter " + sanitizeText(column) + " with:\n")
	if height == 1 {
		return
	}
	visible := max(1, height-2)
	if height == 2 {
		visible = 1
	}
	start := clamp(cursor-visible+1, 0, max(0, len(operators)-visible))
	end := min(start+visible, len(operators))
	for i := start; i < end; i++ {
		operator := operators[i]
		marker := "  "
		if i == cursor {
			marker = "> "
		}
		b.WriteString(marker + browseFilterOperatorLabel(operator) + "\n")
	}
	if height > 2 {
		b.WriteString("Use j/k to choose an operator, then Enter.\n")
	}
}
