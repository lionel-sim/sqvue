package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
	keymap "sqvue/internal/tui/components/keys"
)

type (
	tablesLoadedMsg struct {
		tables []db.Table
		err    error
	}
	rowsLoadedMsg struct {
		columns []db.Column
		rows    [][]string
		err     error
	}
)

type Options struct {
	Client   db.Driver
	PageSize int
	Timeout  time.Duration
}

type Model struct {
	client   db.Driver
	timeout  time.Duration
	tables   []db.Table
	selected int

	columns []db.Column
	rows    [][]string

	status string

	page     int
	pageSize int
	loading  bool
	lastErr  error
	width    int

	keys keymap.Map
}

func New(opts Options) Model {
	return Model{
		client:   opts.Client,
		timeout:  opts.Timeout,
		status:   "loading tables...",
		pageSize: opts.PageSize,
		loading:  true,
		keys:     keymap.Default(),
	}
}

func (m Model) Init() tea.Cmd {
	return loadTablesCmd(m.client, m.timeout)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			m.status = "reloading tables..."
			return m, loadTablesCmd(m.client, m.timeout)

		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.tables)-1 {
				m.selected++
				m.page = 0
				return m.startLoadRows()
			}

		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
				m.page = 0
				return m.startLoadRows()
			}

		case key.Matches(msg, m.keys.PageDown):
			m.page++
			return m.startLoadRows()

		case key.Matches(msg, m.keys.PageUp):
			if m.page > 0 {
				m.page--
				return m.startLoadRows()
			}
		}

	case tablesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("failed to load tables: %v", msg.err)
			m.lastErr = msg.err
			return m, nil
		}
		m.tables = msg.tables
		m.status = fmt.Sprintf("found %d tables", len(m.tables))
		if len(m.tables) > 0 {
			m.selected = 0
			m.page = 0
			return m.startLoadRows()
		}

	case rowsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("rows error: %v", msg.err)
			m.lastErr = msg.err
			return m, nil
		}
		m.columns = msg.columns
		m.rows = msg.rows
		if t := m.currentTable(); t != nil {
			m.status = fmt.Sprintf("%s page %d (%d rows)", t.String(), m.page+1, len(m.rows))
		}
	}

	return m, nil
}

func (m Model) View() string {
	return Render(m)
}

func (m *Model) currentTable() *db.Table {
	if m.selected < 0 || m.selected >= len(m.tables) {
		return nil
	}
	return &m.tables[m.selected]
}

func loadTablesCmd(c db.Driver, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		tables, err := c.ListTables(ctx, "public")
		return tablesLoadedMsg{tables: tables, err: err}
	}
}

func loadRowsCmd(c db.Driver, tbl db.Table, limit, offset int, timeout time.Duration) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cols, rows, err := c.Rows(ctx, tbl, limit, offset)
		return rowsLoadedMsg{columns: cols, rows: rows, err: err}
	}
}

func (m Model) startLoadRows() (tea.Model, tea.Cmd) {
	if t := m.currentTable(); t != nil {
		m.loading = true
		m.status = fmt.Sprintf("loading %s page %d...", t.String(), m.page+1)
		offset := m.page * m.pageSize
		return m, loadRowsCmd(m.client, *t, m.pageSize, offset, m.timeout)
	}
	return m, nil
}

func Render(m Model) string {
	var b strings.Builder

	b.WriteString("Tables (j/k to navigate, pgup/pgdown to page, q to quit)\n")
	if len(m.tables) == 0 {
		b.WriteString("  (no tables found)\n")
	} else {
		for i, t := range m.tables {
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			b.WriteString(prefix + t.String() + "\n")
		}
	}

	b.WriteString("\n")
	if m.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if len(m.columns) > 0 {
		widths := layoutColumns(m.width, m.columns, m.rows)
		for i, col := range m.columns {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(formatCell(col.Name, widths[i]))
		}
		b.WriteString("\n")

		for _, row := range m.rows {
			for i, cell := range row {
				if i > 0 {
					b.WriteString(" | ")
				}
				b.WriteString(formatCell(cell, widths[i]))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("(no rows to display)\n")
	}

	b.WriteString("\n")
	b.WriteString(rightAlign(m.width, "Status: "+m.status))

	return b.String()
}

const (
	maxColWidth  = 40
	minColWidth  = 2
	colSep       = " | "
)

// layoutColumns distributes the terminal width across columns, proportional to
// their natural content width. Returns a width per column; -1 means "no
// truncation" when the terminal width is unknown. Widths may exceed the natural
// width of a column's content so padded cells align the separators.
func layoutColumns(width int, cols []db.Column, rows [][]string) []int {
	widths := make([]int, len(cols))
	if len(cols) == 0 {
		return widths
	}
	if width <= 0 {
		for i := range widths {
			widths[i] = -1
		}
		return widths
	}

	avail := width - len(colSep)*(len(cols)-1)
	if avail <= 0 {
		return widths
	}

	natural := make([]int, len(cols))
	total := 0
	for i, c := range cols {
		n := len(c.Name)
		for _, r := range rows {
			if i < len(r) && len(r[i]) > n {
				n = len(r[i])
			}
		}
		if n > maxColWidth {
			n = maxColWidth
		}
		if n < minColWidth {
			n = minColWidth
		}
		natural[i] = n
		total += n
	}

	if total <= avail {
		return natural
	}

	remaining := avail
	for i, n := range natural {
		w := n * avail / total
		if w < minColWidth {
			w = minColWidth
		}
		if w > n {
			w = n
		}
		widths[i] = w
		remaining -= w
	}
	for remaining > 0 {
		for i := range widths {
			if remaining <= 0 {
				break
			}
			if widths[i] < natural[i] {
				widths[i]++
				remaining--
			}
		}
	}
	return widths
}

// formatCell truncates s to width (with an ellipsis) or pads it to width so
// columns stay aligned. Content that fits exactly is left as-is, and a
// negative width returns s unchanged.
func formatCell(s string, w int) string {
	if w < 0 {
		return s
	}
	if w == 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > w {
		if w == 1 {
			return "…"
		}
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}

func rightAlign(width int, s string) string {
	if width <= len(s) {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}
