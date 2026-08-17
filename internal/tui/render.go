package tui

import (
	"strings"

	"sqvue/internal/db"
)

const (
	maxColWidth = 40
	minColWidth = 2
	colSep      = " | "
)

// =========================================================================
// RENDER
// =========================================================================

func Render(m Model) string {
	var b strings.Builder

	renderTableList(&b, m.tables, m.selected)
	b.WriteString("\n")
	if m.loading {
		b.WriteString("Loading...\n")
	} else {
		renderRows(&b, m.columns, m.rows, m.width)
	}
	b.WriteString("\n")
	b.WriteString(rightAlign(m.width, "Status: "+m.status))

	return b.String()
}

func renderTableList(b *strings.Builder, tables []db.Table, selected int) {
	b.WriteString("Tables (j/k to navigate, pgup/pgdown to page, q to quit)\n")
	if len(tables) == 0 {
		b.WriteString("  (no tables found)\n")
		return
	}
	for i, t := range tables {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		b.WriteString(prefix + t.String() + "\n")
	}
}

func renderRows(b *strings.Builder, columns []db.Column, rows [][]string, width int) {
	if len(columns) == 0 {
		b.WriteString("(no rows to display)\n")
		return
	}
	widths := layoutColumns(width, columns, rows)
	b.WriteString(formatRow(columnNames(columns), widths) + "\n")
	for _, row := range rows {
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
