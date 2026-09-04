package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
)

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
		if c.ForeignKey != nil {
			names[i] += " [FK]"
		}
	}
	return names
}

// renderDescriptions renders column metadata using the same column layout
// machinery as the rows.
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
		{Name: "references"},
	}
	rows := make([][]string, len(cols))
	for i, c := range cols {
		def := ""
		if c.Default != nil {
			def = *c.Default
		}
		rows[i] = []string{c.Name, c.DataType, yesNo(c.Nullable), def, yesNo(c.IsPrimary), foreignKeyLabel(c.ForeignKey)}
	}
	widths := descriptionWidths(width, headers, rows)
	b.WriteString(formatRow(columnNames(headers), widths) + "\n")
	for i, row := range rows {
		if maxRows >= 0 && i >= maxRows {
			break
		}
		b.WriteString(formatRow(row, widths) + "\n")
	}
}

// descriptionWidths favors the references column so foreign-key targets stay
// useful on typical terminal widths. Other metadata can tolerate truncation
// better because its headers make the meaning clear.
func descriptionWidths(width int, headers []db.Column, rows [][]string) []int {
	if len(headers) == 0 || width <= 0 {
		return layoutColumns(width, headers, rows)
	}

	avail := width - len(colSep)*(len(headers)-1)
	if avail <= 0 {
		return make([]int, len(headers))
	}

	// The descriptions view has five fixed metadata columns followed by
	// references. Reserve at least half of the available space for the latter,
	// while leaving enough room to show the metadata headers.
	const referenceIndex = 5
	if len(headers) <= referenceIndex {
		return layoutColumns(width, headers, rows)
	}
	minimumMetadataWidth := 0
	for i := range headers[:referenceIndex] {
		minimumMetadataWidth += max(ansi.StringWidth(headers[i].Name), minColWidth)
	}
	referenceNaturalWidth := cellWidth(referenceIndex, headers[referenceIndex].Name, rows)
	referenceWidth := min(referenceNaturalWidth, max(avail/2, avail-minimumMetadataWidth))

	metadataNaturalWidths := naturalWidths(headers[:referenceIndex], rows)
	metadataWidths := scaleWidths(metadataNaturalWidths, avail-referenceWidth)
	return append(metadataWidths, referenceWidth)
}

func foreignKeyLabel(foreignKey *db.ForeignKey) string {
	if foreignKey == nil {
		return ""
	}

	parts := make([]string, 0, 3)
	if foreignKey.Schema != "" {
		parts = append(parts, foreignKey.Schema)
	}
	if foreignKey.Table != "" {
		parts = append(parts, foreignKey.Table)
	}
	if foreignKey.Column != "" {
		parts = append(parts, foreignKey.Column)
	}
	return strings.Join(parts, ".")
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
	headerNames := columnNames(cols)
	for i, name := range headerNames {
		natural[i] = clamp(cellWidth(i, name, rows), minColWidth, maxColWidth)
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
