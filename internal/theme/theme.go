package theme

import "github.com/charmbracelet/lipgloss"

// Palette maps readable colour names to ANSI 256-colour values. Keep all
// terminal colour choices here so the theme can be adjusted in one place.
var Palette = map[string]lipgloss.Color{
	"blue":       "69",
	"cyan":       "86",
	"gray":       "245",
	"pink":       "204",
	"light_blue": "111",
	"charcoal":   "235",
}

var (
	Title     = lipgloss.NewStyle().Bold(true).Foreground(Palette["blue"])
	Selected  = lipgloss.NewStyle().Bold(true).Foreground(Palette["cyan"])
	ActiveRow = lipgloss.NewStyle().
			Bold(true).
			Foreground(Palette["cyan"]).
			Background(Palette["charcoal"])
	ActiveCell = lipgloss.NewStyle().
			Bold(true).
			Foreground(Palette["charcoal"]).
			Background(Palette["light_blue"])
	Muted  = lipgloss.NewStyle().Foreground(Palette["gray"])
	Error  = lipgloss.NewStyle().Foreground(Palette["pink"])
	Status = lipgloss.NewStyle().Foreground(Palette["light_blue"])
	Dialog = lipgloss.NewStyle().
		Background(Palette["charcoal"]).
		Padding(1, 2)
	DialogBorder = lipgloss.NewStyle().
			Foreground(Palette["blue"]).
			Background(Palette["charcoal"])
)
