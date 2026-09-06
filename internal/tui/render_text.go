package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

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
