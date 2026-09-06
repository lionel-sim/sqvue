package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/config"
	"sqvue/internal/db"
	"sqvue/internal/theme"
)

type Options struct {
	Client          db.Driver
	Timeout         time.Duration
	ExportDirectory string
	Theme           theme.Theme
	QueryStore      *config.QueryStore
	ProfileName     string
	Profiles        []ConnectionProfile
}

// ConnectionProfile contains the connection data needed for an in-app switch.
// Only Name is ever rendered by the TUI.
type ConnectionProfile struct {
	Name    string
	Config  db.ConnectConfig
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
	keys            Map
	exportDirectory string
	theme           theme.Theme
	queryStore      *config.QueryStore
	profileName     string
	profiles        []ConnectionProfile
	profileCursor   int
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
	browseSort                 db.SortSpec
	visibleColumns             []bool
	visibleColumnKey           string
	tableStream                tableStreamState
	queryState
}
type tableStreamState struct {
	stream   db.RowStream
	cancel   func()
	key      string
	nextPage int
	pending  [][]string
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
	querySQL                     string
	querySourceSQL               string
	queryPreserveEditor          bool
	queryBaseRows                [][]string
	querySort                    db.SortSpec
	queryStreaming               bool
	queryStream                  db.QueryRowStream
	queryStreamCancel            func()
	queryStreamNextPage          int
	queryStreamPending           [][]string
	historyIndex                 int
	historyDraft                 string
	savedQueryNames              []string
	savedQueryCursor             int
	savedQueryRename             string
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
	overlayExport
	overlayBackupScope
	overlayBackupPath
	overlayCellEdit
	overlayCellEditConfirm
	overlaySaveQueryName
	overlaySavedQueries
	overlayRenameQuery
	overlayDeleteQueryConfirm
	overlayProfilePicker
)

type overlayState struct {
	activeOverlay                            overlayMode
	filterInput                              textinput.Model
	filterPrevious                           string
	browseFilterInput                        textinput.Model
	browseFilterOperator                     db.FilterOperator
	browseFilterColumn                       string
	browseFilterCursor                       int
	sqlInput                                 textarea.Model
	exportInput                              textinput.Model
	exportFormat                             exportFormat
	backupInput                              textinput.Model
	cellEditInput                            textinput.Model
	queryNameInput                           textinput.Model
	cellEditColumn, cellEditOriginal         string
	cellEditRefreshPending                   bool
	help                                     help.Model
	columnCursor, columnScroll, detailScroll int
	backupScopeCursor                        int
}
type viewportState struct{ width, height int }
type loadState struct {
	status, copyStatusKind                                          string
	loading                                                         bool
	lastErr                                                         error
	loadID, copyStatusID, exportID, backupID, updateID, reconnectID uint64
}

func New(opts Options) Model {
	styles := opts.Theme
	if styles.DialogBackground == "" {
		styles = theme.Default()
	}
	filter := newThemedTextInput(styles)
	filter.Prompt = "Filter tables: "
	filter.Placeholder = "type to search"
	browseFilter := newThemedTextInput(styles)
	browseFilter.Placeholder = "value"
	sql := newThemedTextarea(styles)
	sql.Prompt = "SQL> "
	sql.Placeholder = "SELECT * FROM ..."
	sql.CharLimit = 0
	sql.SetHeight(5)
	export := newThemedTextInput(styles)
	export.Prompt = "Export CSV to: "
	export.Placeholder = "path/to/results.csv"
	export.CharLimit = 0
	backup := newThemedTextInput(styles)
	backup.Prompt = "Back up to: "
	backup.Placeholder = "path/to/backup"
	backup.CharLimit = 0
	cellEdit := newThemedTextInput(styles)
	cellEdit.CharLimit = 0
	queryName := newThemedTextInput(styles)
	queryName.Prompt = "Query name: "
	queryName.Placeholder = "daily report"
	helpModel := help.New()
	applyHelpTheme(&helpModel, styles)
	return Model{client: opts.Client, timeout: opts.Timeout,
		resultState:  resultState{pageSize: maxPageSize, rowCounts: make(map[string]int64), browseRowCounts: make(map[string]int64), queryState: queryState{historyIndex: -1}},
		overlayState: overlayState{filterInput: filter, browseFilterInput: browseFilter, sqlInput: sql, exportInput: export, backupInput: backup, cellEditInput: cellEdit, queryNameInput: queryName, help: helpModel},
		loadState:    loadState{status: "loading tables...", loading: true}, keys: Default(), exportDirectory: opts.ExportDirectory, theme: styles,
		queryStore: opts.QueryStore, profileName: opts.ProfileName, profiles: append([]ConnectionProfile(nil), opts.Profiles...),
	}
}

func newThemedTextInput(styles theme.Theme) textinput.Model {
	input := textinput.New()
	input.PromptStyle = styles.Title
	input.TextStyle = styles.Selected
	input.PlaceholderStyle = styles.Muted
	input.Cursor.Style = styles.ActiveCell
	input.Cursor.TextStyle = styles.Selected
	return input
}

func newThemedTextarea(styles theme.Theme) textarea.Model {
	input := textarea.New()
	input.FocusedStyle.Prompt = styles.Title
	input.FocusedStyle.Text = styles.Selected
	input.FocusedStyle.Placeholder = styles.Muted
	input.FocusedStyle.CursorLine = styles.ActiveCell
	input.BlurredStyle.Prompt = styles.Title
	input.BlurredStyle.Text = styles.Selected
	input.BlurredStyle.Placeholder = styles.Muted
	input.CharLimit = 0
	input.ShowLineNumbers = true
	return input
}

func applyHelpTheme(helpModel *help.Model, styles theme.Theme) {
	helpModel.Styles.ShortKey = styles.Selected
	helpModel.Styles.ShortDesc = styles.Muted
	helpModel.Styles.ShortSeparator = styles.Muted
	helpModel.Styles.FullKey = styles.Selected
	helpModel.Styles.FullDesc = styles.Muted
	helpModel.Styles.FullSeparator = styles.Muted
	helpModel.Styles.Ellipsis = styles.Muted
}

func (m Model) Init() tea.Cmd { return loadSchemasCmd(m.client, m.timeout, m.loadID) }

// Close releases the current database connection when the program exits.
func (m Model) Close() error {
	m.closeTableStream()
	m.closeQueryStream()
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
