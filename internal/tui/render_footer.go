package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"sqvue/internal/theme"
)

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
