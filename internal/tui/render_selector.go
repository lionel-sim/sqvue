package tui

import (
	"fmt"
	"strings"

	"sqvue/internal/db"
	"sqvue/internal/theme"
)

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
		name := sanitizeText(tables[i].Name)
		if tables[i].Type != "" && tables[i].Type != "table" {
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
