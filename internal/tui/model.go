package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

type Options struct {
	Client  db.Driver
	Timeout time.Duration
}
type viewMode int

const (
	modeValues viewMode = iota
	modeDescriptions
)

type Model struct {
	client  db.Driver
	timeout time.Duration
	browserState
	resultState
	gridState
	overlayState
	viewportState
	loadState
	keys Map
}

type browserState struct {
	schemas                            []db.Schema
	schema, schemaCursor, schemaScroll int
	allTables, tables                  []db.Table
	selected, scroll                   int
}
type resultState struct {
	mode                       viewMode
	columns                    []db.Column
	rows                       [][]string
	tableInfo                  db.TableInfo
	page, pageSize             int
	hasNextPage                bool
	rowCounts, browseRowCounts map[string]int64
	browseFilters              []db.RowFilter
	visibleColumns             []bool
	visibleColumnKey           string
	queryState
}
type gridState struct {
	focused                                             bool
	rowCursor, cellCursor, countPrefix, pendingRowMoves int
}
type queryState struct {
	queryActive                  bool
	queryRows                    [][]string
	queryDuration, queryAffected int64
	queryTruncated               bool
}
type overlayMode uint8

const (
	overlayNone overlayMode = iota
	overlayHelp
	overlaySchemaPicker
	overlayFilter
	overlayBrowseFilterOperator
	overlayBrowseFilter
	overlaySQL
	overlayColumnPicker
	overlayRowDetail
)

type overlayState struct {
	activeOverlay                            overlayMode
	filterInput                              textinput.Model
	filterPrevious                           string
	browseFilterInput                        textinput.Model
	browseFilterOperator                     db.FilterOperator
	browseFilterColumn                       string
	browseFilterCursor                       int
	sqlInput                                 textinput.Model
	help                                     help.Model
	columnCursor, columnScroll, detailScroll int
}
type viewportState struct{ width, height int }
type loadState struct {
	status, copyStatusKind string
	loading                bool
	lastErr                error
	loadID, copyStatusID   uint64
}

func New(opts Options) Model {
	filter := textinput.New()
	filter.Prompt = "Filter tables: "
	filter.Placeholder = "type to search"
	browseFilter := textinput.New()
	browseFilter.Placeholder = "value"
	sql := textinput.New()
	sql.Prompt = "SQL> "
	sql.Placeholder = "SELECT * FROM ..."
	sql.CharLimit = 0
	return Model{client: opts.Client, timeout: opts.Timeout,
		resultState:  resultState{pageSize: maxPageSize, rowCounts: make(map[string]int64), browseRowCounts: make(map[string]int64)},
		overlayState: overlayState{filterInput: filter, browseFilterInput: browseFilter, sqlInput: sql, help: help.New()},
		loadState:    loadState{status: "loading tables...", loading: true}, keys: Default(),
	}
}

func (m Model) Init() tea.Cmd { return loadSchemasCmd(m.client, m.timeout, m.loadID) }
