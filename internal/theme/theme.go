package theme

import "github.com/charmbracelet/lipgloss"

var (
	Title    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	Muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Error    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	Status   = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	Dialog   = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Padding(1, 2)
	DialogBorder = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Background(lipgloss.Color("235"))
)
