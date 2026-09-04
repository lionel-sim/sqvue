package keys

import "github.com/charmbracelet/bubbles/key"

// Map holds all key bindings used across the TUI.
type Map struct {
	Quit             key.Binding
	Refresh          key.Binding
	Up               key.Binding
	Down             key.Binding
	PageUp           key.Binding
	PageDown         key.Binding
	ShowDescriptions key.Binding
	ShowValues       key.Binding
	Schema           key.Binding
	Filter           key.Binding
	Confirm          key.Binding
	Help             key.Binding
}

// Default returns the default key bindings for navigation and quitting.
func Default() Map {
	return Map{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c", "esc"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh tables"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("PgUp/b", "prev page"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("PgDn/f", "next page"),
		),
		ShowDescriptions: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "show columns"),
		),
		ShowValues: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "show rows"),
		),
		Schema: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "switch schema"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter tables"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

func (m Map) ShortHelp() []key.Binding {
	return []key.Binding{m.Help, m.Schema, m.Filter, m.ShowDescriptions, m.Refresh, m.Quit}
}

func (m Map) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.Up, m.Down, m.PageUp, m.PageDown}, {m.Schema, m.Filter, m.ShowDescriptions, m.ShowValues}, {m.Refresh, m.Quit}}
}
