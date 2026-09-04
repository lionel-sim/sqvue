package theme

import "github.com/charmbracelet/lipgloss"

var (
	Title    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	Muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Error    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	Status   = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
)
