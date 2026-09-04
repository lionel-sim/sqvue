package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
	"sqvue/internal/theme"
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

// =========================================================================
// RENDER
// =========================================================================

func Render(m Model) string {
	if m.showHelp {
		return renderHelpModal(m)
	}
	return renderMain(m)
}

func renderMain(m Model) string {
	var b strings.Builder

	if m.showSchemas {
		renderSchemaList(&b, m.schemas, m.schemaCursor, m.schemaScroll)
	} else {
		renderTableList(&b, m.currentSchema(), m.tables, m.selected, m.scroll, m.filterInput.Value())
	}
	b.WriteString("\n")
	if m.showColumns {
		renderColumnPicker(&b, m.columns, m.visibleColumns, m.columnCursor, m.columnScroll, m.columnPickerHeight())
	} else if m.sqlMode {
		b.WriteString(m.sqlInput.View() + "\n")
	} else if m.filtering {
		b.WriteString(m.filterInput.View() + "\n")
	} else if m.showSchemas {
		b.WriteString("Use j/k to choose a schema, then Enter to load its tables.\n")
	} else if m.loading {
		b.WriteString("Loading...\n")
	} else if m.mode == modeDescriptions {
		renderDescriptions(&b, m.tableInfo.Columns, m.width, m.height-reservedRows)
	} else {
		renderVisibleRows(&b, m.columns, m.rows, m.visibleColumns, m.width, m.height-reservedRows)
	}
	padToFooter(&b, m.height)
	b.WriteString(renderFooter(m))

	return b.String()
}

func renderHelpModal(m Model) string {
	background := m
	background.showHelp = false
	base := renderMain(background)
	if m.width <= 0 || m.height <= 0 {
		return renderHelpDialog(m)
	}

	dialog := renderHelpDialog(m)
	dialogLines := strings.Split(dialog, "\n")
	popupWidth := ansi.StringWidth(dialogLines[0])
	left := max(0, (m.width-popupWidth)/2)
	top := max(0, (m.height-len(dialogLines))/2)

	baseLines := strings.Split(base, "\n")
	for len(baseLines) < m.height {
		baseLines = append(baseLines, "")
	}
	for i, line := range dialogLines {
		row := top + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = padRenderLine(baseLines[row], m.width)
		baseLines[row] = ansi.Cut(baseLines[row], 0, left) + line + ansi.Cut(baseLines[row], left+popupWidth, m.width)
	}
	return strings.Join(baseLines, "\n")
}

func padRenderLine(line string, width int) string {
	if padding := width - ansi.StringWidth(line); padding > 0 {
		return line + strings.Repeat(" ", padding)
	}
	return line
}

func renderHelpDialog(m Model) string {
	panelBackground := lipgloss.Color("235")
	width := 62
	if m.width > 0 {
		width = min(width, max(10, m.width-6))
	}
	m.help.Width = width
	m.help.ShowAll = true
	m.help.Styles.FullKey = m.help.Styles.FullKey.Background(panelBackground)
	m.help.Styles.FullDesc = m.help.Styles.FullDesc.Background(panelBackground)
	m.help.Styles.FullSeparator = m.help.Styles.FullSeparator.Background(panelBackground)
	helpText := strings.ReplaceAll(m.help.View(m.keys), "\x1b[0m", "\x1b[0m\x1b[48;5;235m")
	dismiss := theme.Muted.Background(panelBackground).Render("Press any key to return.")
	content := helpText + "\n\n" + dismiss
	panelLines := strings.Split(theme.Dialog.Width(width).Render(content), "\n")
	panelWidth := ansi.StringWidth(panelLines[0])
	return titledDialog(panelLines, panelWidth, "Keyboard shortcuts")
}

func titledDialog(lines []string, width int, title string) string {
	titleEdge := "━ " + title + " "
	top := theme.DialogBorder.Render("┏" + titleEdge + strings.Repeat("━", max(0, width-ansi.StringWidth(titleEdge))) + "┓")
	bottom := theme.DialogBorder.Render("┗" + strings.Repeat("━", width) + "┛")
	for i, line := range lines {
		lines[i] = theme.DialogBorder.Render("┃") + line + theme.DialogBorder.Render("┃")
	}
	return top + "\n" + strings.Join(lines, "\n") + "\n" + bottom
}

func padToFooter(b *strings.Builder, height int) {
	if height <= 0 {
		return
	}
	contentLines := strings.Count(b.String(), "\n")
	for contentLines < height-footerRows {
		b.WriteByte('\n')
		contentLines++
	}
}

func renderFooter(m Model) string {
	bindings := "s schema | / filter | j/k navigate | d columns | y rows | q quit"
	status := "Status: " + sanitizeText(m.status)
	gap := m.width - 1 - ansi.StringWidth(bindings) - ansi.StringWidth(status)
	if gap < 1 {
		gap = 1
	}
	styledStatus := theme.Status.Render(status)
	if m.lastErr != nil {
		styledStatus = theme.Error.Render(status)
	}
	return theme.Muted.Render(bindings) + strings.Repeat(" ", gap) + styledStatus
}

func renderTableList(b *strings.Builder, schema string, tables []db.Table, selected, offset int, filter string) {
	header := fmt.Sprintf("Tables: %s", sanitizeText(schema))
	if filter != "" {
		header += fmt.Sprintf(" [filter: %s]", sanitizeText(filter))
	}
	b.WriteString(theme.Title.Render(header) + "\n")
	if len(tables) == 0 {
		b.WriteString("  (no tables found)\n")
		for i := 1; i < tableListHeight; i++ {
			b.WriteByte('\n')
		}
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
		name := sanitizeText(tables[i].String())
		if tables[i].Type != "" {
			name += " [" + tables[i].Type + "]"
		}
		line := prefix + name
		if i == selected {
			line = theme.Selected.Render(line)
		}
		b.WriteString(line + "\n")
	}
	for i := end - offset; i < tableListHeight; i++ {
		b.WriteByte('\n')
	}
}

func renderSchemaList(b *strings.Builder, schemas []db.Schema, selected, offset int) {
	b.WriteString(theme.Title.Render("Schemas") + "\n")
	end := min(offset+tableListHeight, len(schemas))
	for i := offset; i < end; i++ {
		schema := schemas[i]
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		line := prefix + sanitizeText(schema.Name)
		if i == selected {
			line = theme.Selected.Render(line)
		}
		b.WriteString(line + "\n")
	}
	for i := end - offset; i < tableListHeight; i++ {
		b.WriteByte('\n')
	}
}

func renderColumnPicker(b *strings.Builder, columns []db.Column, visible []bool, cursor, offset, maxRows int) {
	b.WriteString(theme.Title.Render("Visible columns (Space toggle, Enter done)") + "\n")
	end := min(offset+maxRows, len(columns))
	for i := offset; i < end; i++ {
		mark := "[ ]"
		if i < len(visible) && visible[i] {
			mark = "[x]"
		}
		line := fmt.Sprintf("%s %s", mark, sanitizeText(columns[i].Name))
		if i == cursor {
			line = theme.Selected.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
}

// renderRows writes the column header and up to maxRows data rows. A negative
// maxRows (terminal height unknown) shows all rows without clipping.
func renderRows(b *strings.Builder, columns []db.Column, rows [][]string, width, maxRows int) {
	renderVisibleRows(b, columns, rows, nil, width, maxRows)
}

func renderVisibleRows(b *strings.Builder, columns []db.Column, rows [][]string, visible []bool, width, maxRows int) {
	columns, rows = visibleData(columns, rows, visible)
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

func visibleData(columns []db.Column, rows [][]string, visible []bool) ([]db.Column, [][]string) {
	if len(visible) == 0 {
		return columns, rows
	}
	indexes := make([]int, 0, len(columns))
	for i := range columns {
		if i < len(visible) && visible[i] {
			indexes = append(indexes, i)
		}
	}
	filteredColumns := make([]db.Column, len(indexes))
	filteredRows := make([][]string, len(rows))
	for i, index := range indexes {
		filteredColumns[i] = columns[index]
	}
	for i, row := range rows {
		filteredRows[i] = make([]string, 0, len(indexes))
		for _, index := range indexes {
			if index < len(row) {
				filteredRows[i] = append(filteredRows[i], row[index])
			}
		}
	}
	return filteredColumns, filteredRows
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
	n := ansi.StringWidth(sanitizeText(name))
	for _, r := range rows {
		if col < len(r) && ansi.StringWidth(sanitizeText(r[col])) > n {
			n = ansi.StringWidth(sanitizeText(r[col]))
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
	s = sanitizeText(s)
	if w < 0 {
		return s
	}
	if w == 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		return ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-ansi.StringWidth(s))
}

func rightAlign(width int, s string) string {
	if width <= ansi.StringWidth(s) {
		return s
	}
	return strings.Repeat(" ", width-ansi.StringWidth(s)) + s
}

func sanitizeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case 0x1b:
			b.WriteString(`\x1b`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
