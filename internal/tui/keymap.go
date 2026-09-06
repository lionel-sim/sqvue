package tui

import "github.com/charmbracelet/bubbles/key"

// Map holds all key bindings used across the TUI.
type Map struct {
	Quit              key.Binding
	Refresh           key.Binding
	Up                key.Binding
	Down              key.Binding
	Left              key.Binding
	Right             key.Binding
	PageUp            key.Binding
	PageDown          key.Binding
	HalfPageUp        key.Binding
	HalfPageDown      key.Binding
	FirstRow          key.Binding
	LastRow           key.Binding
	CopyCell          key.Binding
	CopyRow           key.Binding
	EditCell          key.Binding
	OpenReference     key.Binding
	ShowDescriptions  key.Binding
	ShowValues        key.Binding
	Schema            key.Binding
	Profiles          key.Binding
	Filter            key.Binding
	BrowseFilter      key.Binding
	ClearBrowseFilter key.Binding
	Confirm           key.Binding
	Help              key.Binding
	SQL               key.Binding
	RunSQL            key.Binding
	FormatSQL         key.Binding
	ExplainSQL        key.Binding
	Columns           key.Binding
	ExportCSV         key.Binding
	ExportJSON        key.Binding
	Backup            key.Binding
	Sort              key.Binding
	SaveQuery         key.Binding
	SavedQueries      key.Binding
	Toggle            key.Binding
}

// Default returns the default key bindings for navigation and quitting.
func Default() Map {
	return Map{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
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
		EditCell: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit cell"),
		),
		OpenReference: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open reference"),
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
		Profiles: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "switch profile"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter tables"),
		),
		BrowseFilter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter rows"),
		),
		ClearBrowseFilter: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "clear row filters"),
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
		RunSQL: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("Ctrl+r", "run SQL editor"),
		),
		FormatSQL: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("Ctrl+f", "format SQL editor"),
		),
		ExplainSQL: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("Ctrl+e", "explain SQL editor"),
		),
		Columns: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "choose columns"),
		),
		ExportCSV: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export CSV"),
		),
		ExportJSON: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "export JSON"),
		),
		Backup: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "back up database"),
		),
		Sort: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort active column"),
		),
		SaveQuery: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("Ctrl+s", "save query (editor)"),
		),
		SavedQueries: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("Ctrl+o", "saved queries (editor)"),
		),
		Toggle: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
	}
}

func (m Map) ShortHelp() []key.Binding {
	return []key.Binding{m.Help, m.SQL, m.Columns, m.EditCell, m.ExportCSV, m.ExportJSON, m.Backup, m.Sort, m.SaveQuery, m.SavedQueries, m.Profiles, m.Schema, m.Filter, m.BrowseFilter, m.ClearBrowseFilter, m.Quit}
}

func (m Map) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.Up, m.Down, m.Left, m.Right, m.PageUp, m.PageDown, m.HalfPageUp, m.HalfPageDown, m.FirstRow, m.LastRow}, {m.CopyCell, m.CopyRow, m.EditCell, m.OpenReference, m.BrowseFilter, m.ClearBrowseFilter, m.Sort, m.SQL, m.RunSQL, m.FormatSQL, m.ExplainSQL, m.Columns, m.ExportCSV, m.ExportJSON, m.Backup, m.SaveQuery, m.SavedQueries, m.Profiles, m.Schema, m.Filter, m.ShowDescriptions, m.ShowValues}, {m.Refresh, m.Quit}}
}
