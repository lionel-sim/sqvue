package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
	"sqvue/internal/theme"
)

func renderTableList(b *strings.Builder, styles theme.Theme, terminalWidth int, schema string, tables []db.Table, selected, offset int, filter string) {
	title := fmt.Sprintf("Tables: %s", sanitizeText(schema))
	if filter != "" {
		title += fmt.Sprintf(" [filter: %s]", sanitizeText(filter))
	}
	width := tableListWidth(terminalWidth, title)
	lines := make([]string, 0, tableListHeight)
	if len(tables) == 0 {
		lines = append(lines, "  (no tables found)")
		for i := 1; i < tableListHeight; i++ {
			lines = append(lines, "")
		}
		b.WriteString(renderTitledPanel(styles, lines, width, title))
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
		name := sanitizeText(tables[i].Name)
		if tables[i].Type != "" && tables[i].Type != "table" {
			name += " [" + tables[i].Type + "]"
		}
		line := ansi.Truncate(prefix+name, width, "…")
		if i == selected {
			line = styles.Selected.Render(line)
		}
		lines = append(lines, line)
	}
	for i := end - offset; i < tableListHeight; i++ {
		lines = append(lines, "")
	}
	b.WriteString(renderTitledPanel(styles, lines, width, title))
}

func tableListWidth(terminalWidth int, title string) int {
	minimum := max(12, ansi.StringWidth(title)+2)
	if terminalWidth <= 0 {
		return minimum
	}
	return max(1, terminalWidth-2)
}

func renderColumnPicker(b *strings.Builder, styles theme.Theme, columns []db.Column, visible []bool, cursor, offset, maxRows int) {
	b.WriteString(styles.Title.Render("Visible columns (Space toggle, Enter done)") + "\n")
	end := min(offset+maxRows, len(columns))
	for i := offset; i < end; i++ {
		mark := "[ ]"
		if i < len(visible) && visible[i] {
			mark = "[x]"
		}
		line := fmt.Sprintf("%s %s", mark, sanitizeText(columns[i].Name))
		if i == cursor {
			line = styles.Selected.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
}
