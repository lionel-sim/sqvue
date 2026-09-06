package theme

import "github.com/charmbracelet/lipgloss"

// Theme contains the semantic styles used throughout the TUI. It is a value
// so every application model can retain its own immutable appearance.
type Theme struct {
	Title, Selected, ActiveRow, ActiveCell lipgloss.Style
	Muted, Error, Status                   lipgloss.Style
	Dialog, DialogBorder                   lipgloss.Style
	DialogBackground                       lipgloss.Color
}

// Default returns sqvue's original ANSI-256 appearance.
func Default() Theme {
	blue := lipgloss.Color("69")
	cyan := lipgloss.Color("86")
	gray := lipgloss.Color("245")
	pink := lipgloss.Color("204")
	lightBlue := lipgloss.Color("111")
	charcoal := lipgloss.Color("235")

	return Theme{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(blue),
		Selected:  lipgloss.NewStyle().Bold(true).Foreground(cyan),
		ActiveRow: lipgloss.NewStyle().Bold(true).Foreground(cyan).Background(charcoal),
		ActiveCell: lipgloss.NewStyle().
			Bold(true).Foreground(charcoal).Background(lightBlue),
		Muted:            lipgloss.NewStyle().Foreground(gray),
		Error:            lipgloss.NewStyle().Foreground(pink),
		Status:           lipgloss.NewStyle().Foreground(lightBlue),
		Dialog:           lipgloss.NewStyle().Background(charcoal).Padding(1, 2),
		DialogBorder:     lipgloss.NewStyle().Foreground(blue).Background(charcoal),
		DialogBackground: charcoal,
	}
}
