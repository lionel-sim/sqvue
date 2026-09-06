package theme

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Theme contains the semantic styles used throughout the TUI. It is a value
// so every application model can retain its own immutable appearance.
type Theme struct {
	Title, Selected, ActiveRow, ActiveCell lipgloss.Style
	Muted, Error, Status                   lipgloss.Style
	Dialog, DialogBorder                   lipgloss.Style
	DialogBackground                       lipgloss.Color
}

// Names returns the supported theme names in their documentation order.
func Names() []string {
	return []string{"default", "light", "high-contrast"}
}

// ByName returns a built-in ANSI-256 theme.
func ByName(name string) (Theme, error) {
	switch name {
	case "", "default":
		return Default(), nil
	case "light":
		return newTheme("25", "31", "242", "160", "25", "255", "231"), nil
	case "high-contrast":
		return newTheme("15", "51", "250", "196", "226", "16", "0"), nil
	default:
		return Theme{}, fmt.Errorf("unknown theme %q (available: default, light, high-contrast)", name)
	}
}

// Default returns sqvue's original ANSI-256 appearance.
func Default() Theme {
	return newTheme("69", "86", "245", "204", "111", "235", "235")
}

func newTheme(primary, accent, muted, errorColor, highlight, surface, highlightText string) Theme {
	primaryColor := lipgloss.Color(primary)
	accentColor := lipgloss.Color(accent)
	mutedColor := lipgloss.Color(muted)
	errorStyleColor := lipgloss.Color(errorColor)
	highlightColor := lipgloss.Color(highlight)
	surfaceColor := lipgloss.Color(surface)
	highlightTextColor := lipgloss.Color(highlightText)

	return Theme{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(primaryColor),
		Selected:  lipgloss.NewStyle().Bold(true).Foreground(accentColor),
		ActiveRow: lipgloss.NewStyle().Bold(true).Foreground(accentColor).Background(surfaceColor),
		ActiveCell: lipgloss.NewStyle().
			Bold(true).Foreground(highlightTextColor).Background(highlightColor),
		Muted:            lipgloss.NewStyle().Foreground(mutedColor),
		Error:            lipgloss.NewStyle().Foreground(errorStyleColor),
		Status:           lipgloss.NewStyle().Foreground(highlightColor),
		Dialog:           lipgloss.NewStyle().Background(surfaceColor).Padding(1, 2),
		DialogBorder:     lipgloss.NewStyle().Foreground(primaryColor).Background(surfaceColor),
		DialogBackground: surfaceColor,
	}
}
