package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/db"
	"sqvue/internal/theme"
)

type tableListRenderOptions struct {
	terminalWidth int
	schema        string
	tables        []db.Table
	selected      int
	offset        int
	filter        string
}

func renderTableList(b *strings.Builder, styles theme.Theme, options tableListRenderOptions) {
	title := fmt.Sprintf("Tables: %s", sanitizeText(options.schema))
	if options.filter != "" {
		title += fmt.Sprintf(" [filter: %s]", sanitizeText(options.filter))
	}
	width := tableListWidth(options.terminalWidth, title)
	lines := make([]string, 0, tableListHeight)
	if len(options.tables) == 0 {
		lines = append(lines, "  (no tables found)")
		for i := 1; i < tableListHeight; i++ {
			lines = append(lines, "")
		}
		b.WriteString(renderTitledPanel(styles, lines, width, title))
		return
	}
	end := options.offset + tableListHeight
	if end > len(options.tables) {
		end = len(options.tables)
	}
	for i := options.offset; i < end; i++ {
		prefix := "  "
		if i == options.selected {
			prefix = "> "
		}
		name := sanitizeText(options.tables[i].Name)
		if options.tables[i].Type != "" && options.tables[i].Type != "table" {
			name += " [" + options.tables[i].Type + "]"
		}
		line := ansi.Truncate(prefix+name, width, "…")
		if i == options.selected {
			line = styles.Selected.Render(line)
		}
		lines = append(lines, line)
	}
	for i := end - options.offset; i < tableListHeight; i++ {
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
