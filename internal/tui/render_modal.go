package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/theme"
)

func renderHelpModal(m Model) string {
	background := m
	background.activeOverlay = overlayNone
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
