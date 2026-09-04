package keys

import "github.com/charmbracelet/bubbles/key"

// Map holds all key bindings used across the TUI.
type Map struct {
	Quit             key.Binding
	Refresh          key.Binding
	Up               key.Binding
	Down             key.Binding
	Left             key.Binding
	Right            key.Binding
	PageUp           key.Binding
	PageDown         key.Binding
	HalfPageUp       key.Binding
	HalfPageDown     key.Binding
	FirstRow         key.Binding
	LastRow          key.Binding
	CopyCell         key.Binding
	CopyRow          key.Binding
	ShowDescriptions key.Binding
	ShowValues       key.Binding
	Schema           key.Binding
	Filter           key.Binding
	Confirm          key.Binding
	Help             key.Binding
	SQL              key.Binding
	Columns          key.Binding
	Toggle           key.Binding
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
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("PgUp/b", "prev page"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("PgDn/f", "next page"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("Ctrl+u", "half page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("Ctrl+d", "half page down"),
		),
		FirstRow: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "first row"),
		),
		LastRow: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "last row"),
		),
		CopyCell: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy cell"),
		),
		CopyRow: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "copy row"),
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
		SQL: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "run SQL"),
		),
		Columns: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "choose columns"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
	}
}

func (m Map) ShortHelp() []key.Binding {
	return []key.Binding{m.Help, m.SQL, m.Columns, m.Schema, m.Filter, m.Quit}
}

func (m Map) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.Up, m.Down, m.Left, m.Right, m.PageUp, m.PageDown, m.HalfPageUp, m.HalfPageDown, m.FirstRow, m.LastRow}, {m.CopyCell, m.CopyRow, m.SQL, m.Columns, m.Schema, m.Filter, m.ShowDescriptions, m.ShowValues}, {m.Refresh, m.Quit}}
}
