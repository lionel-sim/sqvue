package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/theme"
)

func renderHelpModal(m Model) string {
	return renderModalOverMain(m, renderHelpDialog(m))
}

func renderRowDetailModal(m Model) string {
	return renderModalOverMain(m, renderRowDetailDialog(m))
}

func renderModalOverMain(m Model, dialog string) string {
	background := m
	background.activeOverlay = overlayNone
	base := renderMain(background)
	if m.width <= 0 || m.height <= 0 {
		return dialog
	}

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
	panelBackground := theme.Palette["charcoal"]
	width := 62
	if m.width > 0 {
		width = min(width, max(10, m.width-6))
	}
	m.help.Width = width
	m.help.ShowAll = true
	m.help.Styles.FullKey = m.help.Styles.FullKey.Background(panelBackground)
	m.help.Styles.FullDesc = m.help.Styles.FullDesc.Background(panelBackground)
	m.help.Styles.FullSeparator = m.help.Styles.FullSeparator.Background(panelBackground)
	helpText := strings.ReplaceAll(m.help.View(m.helpKeyMap()), "\x1b[0m", "\x1b[0m\x1b[48;5;235m")
	dismiss := theme.Muted.Background(panelBackground).Render("Press any key to return.")
	content := helpText + "\n\n" + dismiss
	panelLines := strings.Split(theme.Dialog.Width(width).Render(content), "\n")
	panelWidth := ansi.StringWidth(panelLines[0])
	return titledDialog(panelLines, panelWidth, "Keyboard shortcuts")
}

func renderRowDetailDialog(m Model) string {
	width := detailDialogWidth(m)
	lines := rowDetailLines(m, width)
	start := min(m.detailScroll, max(0, len(lines)-detailContentHeight(m)))
	end := min(start+detailContentHeight(m), len(lines))
	content := strings.Join(lines[start:end], "\n")
	panelLines := strings.Split(theme.Dialog.Width(width).Render(content), "\n")
	panelWidth := ansi.StringWidth(panelLines[0])
	return titledDialog(panelLines, panelWidth, "Row details")
}

func detailDialogWidth(m Model) int {
	if m.width <= 0 {
		return 80
	}
	return min(80, max(20, m.width-6))
}

func detailContentHeight(m Model) int {
	if m.height <= 0 {
		return int(^uint(0) >> 1)
	}
	// The dialog border and vertical padding consume four terminal rows.
	return max(1, m.height-4)
}

func (m Model) maxDetailScroll() int {
	return max(0, len(rowDetailLines(m, detailDialogWidth(m)))-detailContentHeight(m))
}

func rowDetailLines(m Model, width int) []string {
	if m.rowCursor < 0 || m.rowCursor >= len(m.rows) {
		return []string{"(no row selected)"}
	}

	row := m.rows[m.rowCursor]
	contentWidth := max(1, width-4)
	lines := make([]string, 0, len(m.columns))
	for i, column := range m.columns {
		prefix := sanitizeText(column.Name) + ": "
		value := ""
		if i < len(row) {
			value = sanitizeText(row[i])
		}
		valueLines := wrapDetailValue(value, max(1, contentWidth-ansi.StringWidth(prefix)))
		lines = append(lines, prefix+valueLines[0])
		indent := strings.Repeat(" ", ansi.StringWidth(prefix))
		for _, line := range valueLines[1:] {
			lines = append(lines, indent+line)
		}
	}
	if len(lines) == 0 {
		return []string{"(no values)"}
	}
	return lines
}

func wrapDetailValue(value string, width int) []string {
	if value == "" {
		return []string{""}
	}
	lines := make([]string, 0, 1)
	var line strings.Builder
	lineWidth := 0
	for _, r := range value {
		runeWidth := ansi.StringWidth(string(r))
		if lineWidth > 0 && lineWidth+runeWidth > width {
			lines = append(lines, line.String())
			line.Reset()
			lineWidth = 0
		}
		line.WriteRune(r)
		lineWidth += runeWidth
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
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
