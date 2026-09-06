package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/theme"
)

func renderHelpModal(m Model) string {
	return renderModalOverMain(m, renderHelpDialog(m))
}

func renderRowDetailModal(m Model) string {
	return renderModalOverMain(m, renderRowDetailDialog(m))
}

func renderCellEditModal(m Model) string {
	width := detailDialogWidth(m)
	input := m.cellEditInput
	input.Width = max(1, width-ansi.StringWidth(input.Prompt)-4)
	content := input.View() + "\n\n" + m.theme.Muted.Render("Enter reviews the update. Esc cancels. Type NULL to clear the value.")
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(content), "\n")
	return titledDialog(m, panelLines, ansi.StringWidth(panelLines[0]), "Edit cell")
}

func renderCellEditConfirmModal(m Model) string {
	width := detailDialogWidth(m)
	content := "Column: " + sanitizeText(m.cellEditColumn) + "\n" +
		"Current: " + sanitizeText(m.cellEditOriginal) + "\n" +
		"New: " + sanitizeText(m.cellEditInput.Value()) + "\n\n" +
		m.theme.Muted.Render("Enter saves the update. Esc returns to editing.")
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(content), "\n")
	return titledDialog(m, panelLines, ansi.StringWidth(panelLines[0]), "Confirm cell update")
}

func renderSaveQueryNameModal(m Model) string {
	return renderQueryNameModal(m, "Save query", "Enter saves. Esc returns to the SQL prompt.")
}

func renderRenameQueryModal(m Model) string {
	return renderQueryNameModal(m, "Rename saved query", "Enter renames. Esc returns to saved queries.")
}

func renderQueryNameModal(m Model, title, hint string) string {
	width := detailDialogWidth(m)
	input := m.queryNameInput
	input.Width = max(1, width-ansi.StringWidth(input.Prompt)-4)
	content := input.View() + "\n\n" + m.theme.Muted.Render(hint)
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(content), "\n")
	return titledDialog(m, panelLines, ansi.StringWidth(panelLines[0]), title)
}

func renderSavedQueriesModal(m Model) string {
	width := detailDialogWidth(m)
	lines := make([]string, 0, len(m.savedQueryNames)+2)
	if len(m.savedQueryNames) == 0 {
		lines = append(lines, "(no saved queries for this profile)")
	} else {
		for i, name := range m.savedQueryNames {
			marker := "  "
			if i == m.savedQueryCursor {
				marker = "> "
			}
			lines = append(lines, marker+sanitizeText(name))
		}
	}
	lines = append(lines, "", m.theme.Muted.Render("Enter runs · r renames · d deletes · Esc returns"))
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(strings.Join(lines, "\n")), "\n")
	return titledDialog(m, panelLines, ansi.StringWidth(panelLines[0]), "Saved queries")
}

func renderDeleteQueryConfirmModal(m Model) string {
	name := ""
	if m.savedQueryCursor >= 0 && m.savedQueryCursor < len(m.savedQueryNames) {
		name = sanitizeText(m.savedQueryNames[m.savedQueryCursor])
	}
	width := detailDialogWidth(m)
	content := "Delete saved query " + name + "?\n\n" + m.theme.Muted.Render("Enter deletes it. Esc cancels.")
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(content), "\n")
	return titledDialog(m, panelLines, ansi.StringWidth(panelLines[0]), "Delete saved query")
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
	panelBackground := m.theme.DialogBackground
	width := 62
	if m.width > 0 {
		width = min(width, max(10, m.width-6))
	}
	m.help.Width = width
	m.help.ShowAll = true
	m.help.Styles.FullKey = m.help.Styles.FullKey.Background(panelBackground)
	m.help.Styles.FullDesc = m.help.Styles.FullDesc.Background(panelBackground)
	m.help.Styles.FullSeparator = m.help.Styles.FullSeparator.Background(panelBackground)
	helpText := strings.ReplaceAll(m.help.View(m.helpKeyMap()), "\x1b[0m", "\x1b[0m"+backgroundPrefix(panelBackground))
	dismiss := m.theme.Muted.Background(panelBackground).Render("Press any key to return.")
	content := helpText + "\n\n" + dismiss
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(content), "\n")
	panelWidth := ansi.StringWidth(panelLines[0])
	return titledDialog(m, panelLines, panelWidth, "Keyboard shortcuts")
}

func backgroundPrefix(color lipgloss.Color) string {
	styledSpace := lipgloss.NewStyle().Background(color).Render(" ")
	return strings.TrimSuffix(strings.TrimSuffix(styledSpace, "\x1b[0m"), " ")
}

func renderRowDetailDialog(m Model) string {
	width := detailDialogWidth(m)
	lines := rowDetailLines(m, width)
	start := min(m.detailScroll, max(0, len(lines)-detailContentHeight(m)))
	end := min(start+detailContentHeight(m), len(lines))
	content := strings.Join(lines[start:end], "\n")
	panelLines := strings.Split(m.theme.Dialog.Width(width).Render(content), "\n")
	panelWidth := ansi.StringWidth(panelLines[0])
	return titledDialog(m, panelLines, panelWidth, "Row details")
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

func titledDialog(m Model, lines []string, width int, title string) string {
	titleEdge := "━ " + title + " "
	top := m.theme.DialogBorder.Render("┏" + titleEdge + strings.Repeat("━", max(0, width-ansi.StringWidth(titleEdge))) + "┓")
	bottom := m.theme.DialogBorder.Render("┗" + strings.Repeat("━", width) + "┛")
	for i, line := range lines {
		lines[i] = m.theme.DialogBorder.Render("┃") + line + m.theme.DialogBorder.Render("┃")
	}
	return top + "\n" + strings.Join(lines, "\n") + "\n" + bottom
}

// renderTitledPanel renders a fixed-height, terminal-width panel used by the
// top-level table selector.
func renderTitledPanel(styles theme.Theme, lines []string, width int, title string) string {
	width = max(1, width)
	title = ansi.Truncate(title, max(0, width-4), "…")
	titleEdge := "━ " + title + " "
	top := styles.DialogBorder.Render("┏━ ") + styles.Title.Render(title) + styles.DialogBorder.Render(" "+strings.Repeat("━", max(0, width-ansi.StringWidth(titleEdge)))+"┓")
	bottom := styles.DialogBorder.Render("┗" + strings.Repeat("━", width) + "┛")
	contentStyle := styles.Dialog.Padding(0).Width(width)
	for i, line := range lines {
		lines[i] = styles.DialogBorder.Render("┃") + contentStyle.Render(line) + styles.DialogBorder.Render("┃")
	}
	return top + "\n" + strings.Join(lines, "\n") + "\n" + bottom
}
