package tui

import (
	"fmt"
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
	footerRows      = 2
	// reservedRows counts every fixed line above and below the data rows:
	// list header, list, blank gap, column header, and the footer.
	reservedRows = tableListHeader + tableListHeight + gapAfterList + columnsHeader + footerRows
)

// =========================================================================
// RENDER
// =========================================================================

func Render(m Model) string {
	var b strings.Builder

	if m.showSchemas {
		renderSchemaList(&b, m.schemas, m.schema)
	} else {
		renderTableList(&b, m.currentSchema(), m.tables, m.selected, m.scroll, m.filterInput.Value())
	}
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(m.filterInput.View() + "\n")
	} else if m.showSchemas {
		b.WriteString("Use j/k to choose a schema, then Enter to load its tables.\n")
	} else if m.loading {
		b.WriteString("Loading...\n")
	} else if m.mode == modeDescriptions {
		renderDescriptions(&b, m.tableInfo.Columns, m.width, m.height-reservedRows)
	} else {
		renderRows(&b, m.columns, m.rows, m.width, m.height-reservedRows)
	}
	b.WriteString("\n")
	// Right-align one column short of the edge so the final character isn't
	// clipped by terminals at the bottom-right corner.
	b.WriteString(rightAlign(m.width-1, "Status: "+m.status))

	return b.String()
}

func renderTableList(b *strings.Builder, schema string, tables []db.Table, selected, offset int, filter string) {
	header := fmt.Sprintf("Tables: %s (s schema, / filter, j/k navigate, d columns, y rows, q quit)", schema)
	if filter != "" {
		header += fmt.Sprintf(" [filter: %s]", filter)
	}
	b.WriteString(header + "\n")
	if len(tables) == 0 {
		b.WriteString("  (no tables found)\n")
		return
	}
	end := offset + tableListHeight
	if end > len(tables) {
		end = len(tables)
	}
	for i := offset; i < end; i++ {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		b.WriteString(prefix + tables[i].String() + "\n")
	}
}

func renderSchemaList(b *strings.Builder, schemas []db.Schema, selected int) {
	b.WriteString("Schemas\n")
	for i, schema := range schemas {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		b.WriteString(prefix + schema.Name + "\n")
	}
}

// renderRows writes the column header and up to maxRows data rows. A negative
// maxRows (terminal height unknown) shows all rows without clipping.
func renderRows(b *strings.Builder, columns []db.Column, rows [][]string, width, maxRows int) {
	if len(columns) == 0 {
		b.WriteString("(no rows to display)\n")
		return
	}
	widths := layoutColumns(width, columns, rows)
	b.WriteString(formatRow(columnNames(columns), widths) + "\n")
	for i, row := range rows {
		if maxRows >= 0 && i >= maxRows {
			break
		}
		b.WriteString(formatRow(row, widths) + "\n")
	}
}

func columnNames(cols []db.Column) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	return names
}

// renderDescriptions renders column metadata (name, type, nullability,
// default, primary key) using the same column layout machinery as the rows.
func renderDescriptions(b *strings.Builder, cols []db.Column, width, maxRows int) {
	if len(cols) == 0 {
		b.WriteString("(no columns to display)\n")
		return
	}
	headers := []db.Column{
		{Name: "column"},
		{Name: "type"},
		{Name: "nullable"},
		{Name: "default"},
		{Name: "primary"},
	}
	rows := make([][]string, len(cols))
	for i, c := range cols {
		def := ""
		if c.Default != nil {
			def = *c.Default
		}
		rows[i] = []string{
			c.Name,
			c.DataType,
			yesNo(c.Nullable),
			def,
			yesNo(c.IsPrimary),
		}
	}
	widths := layoutColumns(width, headers, rows)
	b.WriteString(formatRow(columnNames(headers), widths) + "\n")
	for i, row := range rows {
		if maxRows >= 0 && i >= maxRows {
			break
		}
		b.WriteString(formatRow(row, widths) + "\n")
	}
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func formatRow(cells []string, widths []int) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = formatCell(cell, widths[i])
	}
	return strings.Join(parts, colSep)
}

// =========================================================================
// layoutColumns
// =========================================================================

// layoutColumns distributes the terminal width across columns, proportional to
// their natural content width. Returns a width per column; -1 means "no
// truncation" when the terminal width is unknown.
func layoutColumns(width int, cols []db.Column, rows [][]string) []int {
	widths := make([]int, len(cols))
	if len(cols) == 0 || width <= 0 {
		for i := range widths {
			widths[i] = -1
		}
		return widths
	}

	avail := width - len(colSep)*(len(cols)-1)
	if avail <= 0 {
		return widths
	}

	natural := naturalWidths(cols, rows)
	if sumInts(natural) <= avail {
		return natural
	}
	return scaleWidths(natural, avail)
}

func naturalWidths(cols []db.Column, rows [][]string) []int {
	natural := make([]int, len(cols))
	for i, c := range cols {
		natural[i] = clamp(cellWidth(i, c.Name, rows), minColWidth, maxColWidth)
	}
	return natural
}

func cellWidth(col int, name string, rows [][]string) int {
	n := len(name)
	for _, r := range rows {
		if col < len(r) && len(r[col]) > n {
			n = len(r[col])
		}
	}
	return n
}

func scaleWidths(natural []int, avail int) []int {
	total := sumInts(natural)
	widths := make([]int, len(natural))
	for i, n := range natural {
		widths[i] = clamp(n*avail/total, minColWidth, n)
	}
	distributeRemainder(widths, natural, avail)
	return widths
}

// distributeRemainder grows columns to absorb any width left over after
// proportional scaling, never exceeding a column's natural width.
func distributeRemainder(widths, natural []int, avail int) {
	remaining := avail - sumInts(widths)
	for remaining > 0 {
		grew := false
		for i, n := range natural {
			if widths[i] < n {
				widths[i]++
				remaining--
				grew = true
				if remaining == 0 {
					break
				}
			}
		}
		if !grew {
			break
		}
	}
}

func sumInts(vals []int) int {
	total := 0
	for _, v := range vals {
		total += v
	}
	return total
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// =========================================================================
// OTHER HELPERS
// =========================================================================

// formatCell truncates s to width (with an ellipsis) or pads it to width so
// columns stay aligned. Content that fits exactly is left as-is, and a
// negative width returns s unchanged.
func formatCell(s string, w int) string {
	if w < 0 {
		return s
	}
	if w == 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > w {
		if w == 1 {
			return "…"
		}
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}

func rightAlign(width int, s string) string {
	if width <= len(s) {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}
