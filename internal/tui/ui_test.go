package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sqvue/internal/config"
	"sqvue/internal/db"
	"sqvue/internal/theme"
)

func testModel() Model {
	m := New(Options{Client: nil, Timeout: 0})
	m.tables = []db.Table{{Schema: "public", Name: "a"}, {Schema: "public", Name: "b"}}
	m.loading = false
	return m
}

func TestNewUsesProvidedTheme(t *testing.T) {
	styles, err := theme.ByName("light")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Theme: styles})
	if m.theme.DialogBackground != styles.DialogBackground {
		t.Fatalf("model theme background = %q, want %q", m.theme.DialogBackground, styles.DialogBackground)
	}
}

func TestNewDefaultsThemeWhenNoneIsProvided(t *testing.T) {
	m := New(Options{})
	if m.theme.DialogBackground != theme.Default().DialogBackground {
		t.Fatalf("model theme background = %q, want default %q", m.theme.DialogBackground, theme.Default().DialogBackground)
	}
}

func TestNewAppliesThemeToInputsAndHelp(t *testing.T) {
	styles, err := theme.ByName("high-contrast")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Theme: styles})
	if m.sqlInput.FocusedStyle.Prompt.GetForeground() != styles.Title.GetForeground() {
		t.Fatal("SQL prompt did not receive the theme title style")
	}
	if _, ok := m.sqlInput.FocusedStyle.CursorLine.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatal("SQL cursor line should not paint a background")
	}
	if m.browseFilterInput.PlaceholderStyle.GetForeground() != styles.Muted.GetForeground() {
		t.Fatal("filter placeholder did not receive the theme muted style")
	}
	if m.help.Styles.FullKey.GetForeground() != styles.Selected.GetForeground() {
		t.Fatal("help keys did not receive the theme selected style")
	}
}

func TestProfilePickerRendersNamesWithoutConnectionDetails(t *testing.T) {
	m := testModel()
	m.profileName = "local"
	m.profiles = []ConnectionProfile{
		{Name: "local", Config: db.ConnectConfig{User: "reader", Password: "super-secret", Database: "app"}, Timeout: time.Second},
		{Name: "reporting", Config: db.ConnectConfig{User: "analyst", Password: "another-secret", Database: "warehouse"}, Timeout: time.Second},
	}
	m, _ = m.openProfilePicker()
	if m.activeOverlay != overlayProfilePicker || m.profileCursor != 0 {
		t.Fatalf("profile picker = overlay %v, cursor %d", m.activeOverlay, m.profileCursor)
	}
	view := Render(m)
	if !strings.Contains(view, "local") || !strings.Contains(view, "reporting") || strings.Contains(view, "super-secret") || strings.Contains(view, "another-secret") {
		t.Fatalf("profile picker rendered connection details: %q", view)
	}
}

func TestProfileReconnectResetsBrowserOnlyAfterSuccessfulConnection(t *testing.T) {
	previous := &fakeDriver{}
	replacement := &fakeDriver{}
	m := testModel()
	m.client, m.profileName, m.reconnectID = previous, "local", 2
	m.schemas = []db.Schema{{Name: "public"}}
	m.tables = []db.Table{{Schema: "public", Name: "accounts"}}
	m.rows = [][]string{{"1"}}
	m.queryActive, m.focused = true, true
	profile := ConnectionProfile{Name: "reporting", Timeout: time.Second}
	m, cmd := m.handleProfileConnected(profileConnectedMsg{reconnectID: 2, profile: profile, client: replacement})
	if cmd == nil || m.client != replacement || m.profileName != "reporting" || len(m.tables) != 0 || len(m.rows) != 0 || m.queryActive || m.focused {
		t.Fatalf("successful reconnect did not reset browser state")
	}

	m = testModel()
	m.client, m.profileName, m.reconnectID = previous, "local", 3
	m.tables = []db.Table{{Schema: "public", Name: "accounts"}}
	m, cmd = m.handleProfileConnected(profileConnectedMsg{reconnectID: 3, profile: profile, err: errors.New("offline")})
	if cmd != nil || m.client != previous || m.profileName != "local" || len(m.tables) != 1 || m.lastErr == nil {
		t.Fatalf("failed reconnect did not preserve the current connection")
	}
}

func TestSQLHistoryNavigationRestoresDraftAndStaysPerProfile(t *testing.T) {
	store, err := config.LoadQueryStore(filepath.Join(t.TempDir(), "queries.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddHistory("work", "select one"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddHistory("work", "select two"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddHistory("other", "select private"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{QueryStore: store, ProfileName: "work"})
	m.sqlInput.SetValue("draft")

	m, handled := m.handleSQLHistoryKey(keyMsg("k"))
	if !handled || m.sqlInput.Value() != "select two" {
		t.Fatalf("first previous history = %q, handled %t", m.sqlInput.Value(), handled)
	}
	m, _ = m.handleSQLHistoryKey(keyMsg("k"))
	if m.sqlInput.Value() != "select one" {
		t.Fatalf("second previous history = %q", m.sqlInput.Value())
	}
	m, _ = m.handleSQLHistoryKey(keyMsg("j"))
	m, _ = m.handleSQLHistoryKey(keyMsg("j"))
	if m.sqlInput.Value() != "draft" || m.historyIndex != -1 {
		t.Fatalf("restored draft = %q, index %d", m.sqlInput.Value(), m.historyIndex)
	}
}

func TestSavedQueryWorkflowPreservesMultilineSQL(t *testing.T) {
	store, err := config.LoadQueryStore(filepath.Join(t.TempDir(), "queries.toml"))
	if err != nil {
		t.Fatal(err)
	}
	client := &fakeDriver{}
	m := New(Options{Client: client, QueryStore: store, ProfileName: "work"})
	m.sqlInput.SetValue("select * from accounts")
	m, _ = m.beginSaveQuery()
	m.queryNameInput.SetValue("accounts")
	m, _ = m.handleSaveQueryNameKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.activeOverlay != overlaySQL {
		t.Fatalf("overlay after save = %v, want SQL", m.activeOverlay)
	}
	if err := store.SaveQuery("work", "accounts", "select *\nfrom accounts"); err != nil {
		t.Fatal(err)
	}

	m, _ = m.openSavedQueries()
	if len(m.savedQueryNames) != 1 || m.savedQueryNames[0] != "accounts" {
		t.Fatalf("saved query names = %#v", m.savedQueryNames)
	}
	m, cmd := m.handleSavedQueriesKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || m.activeOverlay != overlayNone || !m.loading {
		t.Fatalf("saved query did not begin execution: overlay %v, loading %t, command %t", m.activeOverlay, m.loading, cmd != nil)
	}
	if msg := runCmd(cmd); msg == nil {
		t.Fatal("saved query command did not produce a result")
	}
	if client.lastQuery != "select *\nfrom accounts" {
		t.Fatalf("saved query SQL = %q", client.lastQuery)
	}

	m, _ = m.openSavedQueries()
	m, _ = m.handleSavedQueriesKey(keyMsg("r"))
	m.queryNameInput.SetValue("all accounts")
	m, _ = m.handleRenameQueryKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.savedQueryNames; len(got) != 1 || got[0] != "all accounts" {
		t.Fatalf("renamed query names = %#v", got)
	}
	m, _ = m.handleSavedQueriesKey(keyMsg("d"))
	if m.activeOverlay != overlayDeleteQueryConfirm {
		t.Fatalf("delete overlay = %v", m.activeOverlay)
	}
	m, _ = m.handleDeleteQueryConfirmKey(tea.KeyMsg{Type: tea.KeyEnter})
	if got := store.QueryNames("work"); len(got) != 0 {
		t.Fatalf("remaining saved queries = %#v", got)
	}
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func TestMoveSelectionBounds(t *testing.T) {
	m := testModel()
	m.selected = 1
	if got, _ := m.moveSelection(+1); got.selected != 1 {
		t.Fatalf("expected to stay at last, got %d", got.selected)
	}
	m.selected = 0
	if got, _ := m.moveSelection(-1); got.selected != 0 {
		t.Fatalf("expected to stay at first, got %d", got.selected)
	}
	m.selected = 0
	if got, _ := m.moveSelection(+1); got.selected != 1 {
		t.Fatalf("expected to move to 1, got %d", got.selected)
	}
}

func TestChangePageBounds(t *testing.T) {
	m := testModel()
	m.page = 0
	if got, _ := m.changePage(-1); got.page != 0 {
		t.Fatalf("expected page to stay 0, got %d", got.page)
	}
	m.page = 2
	if got, _ := m.changePage(-1); got.page != 1 {
		t.Fatalf("expected page 1, got %d", got.page)
	}
}

func TestUpdateRoutesWindowSize(t *testing.T) {
	got, _ := testModel().Update(tea.WindowSizeMsg{Width: 80})
	if got.(Model).width != 80 {
		t.Fatalf("expected width 80, got %d", got.(Model).width)
	}
}

func TestUpdateQuitKey(t *testing.T) {
	_, cmd := testModel().Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestEscapeDoesNotQuitOutsideOfGridFocus(t *testing.T) {
	_, cmd := testModel().Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("expected Esc to leave the application running")
	}
}

func TestEnterFocusesGridAndEscapeReturnsToTablePicker(t *testing.T) {
	m := testModel()
	m.rows = [][]string{{"one"}}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.focused {
		t.Fatal("expected Enter to focus the data grid")
	}
	if m.rowCursor != 0 {
		t.Fatalf("row cursor = %d, want 0", m.rowCursor)
	}

	m, _ = update(m, keyMsg("j"))
	if m.selected != 0 {
		t.Fatalf("selected table changed while grid was focused: %d", m.selected)
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.focused {
		t.Fatal("expected Esc to return focus to the table picker")
	}
}

func TestEditCellKeyIsContextSensitive(t *testing.T) {
	m := testModel()
	m.focused = true
	m.queryActive = true
	m, _ = update(m, keyMsg("e"))
	if m.status != "editing is unavailable for SQL results" {
		t.Fatalf("SQL edit status = %q", m.status)
	}

	m = testModel()
	m.focused = true
	m, _ = update(m, keyMsg("e"))
	if m.status != "editing requires a table with a primary key" {
		t.Fatalf("table edit status = %q", m.status)
	}

	m.columns = []db.Column{{Name: "id", IsPrimary: true}}
	m.rows = [][]string{{"1"}}
	m, _ = update(m, keyMsg("e"))
	if m.status != "edit cell value" || m.activeOverlay != overlayCellEdit {
		t.Fatalf("primary-key table edit status = %q", m.status)
	}

	m = testModel()
	m.columns = []db.Column{{Name: "id"}}
	m, _ = update(m, keyMsg("e"))
	if m.activeOverlay != overlayExport {
		t.Fatalf("unfocused e overlay = %v, want CSV export", m.activeOverlay)
	}
}

func TestCellEditRequestUsesCompositePrimaryKeyAndNull(t *testing.T) {
	m := testModel()
	m.columns = []db.Column{{Name: "tenant", IsPrimary: true}, {Name: "id", IsPrimary: true}, {Name: "note"}}
	m.rows = [][]string{{"north", "7", "before"}}
	m.cellCursor = 2
	m.cellEditInput.SetValue("NULL")
	request, err := m.cellUpdateRequest()
	if err != nil {
		t.Fatal(err)
	}
	if request.Value != nil || len(request.PrimaryKey) != 2 || request.PrimaryKey[0] != (db.PrimaryKeyValue{Column: "tenant", Value: "north"}) || request.PrimaryKey[1] != (db.PrimaryKeyValue{Column: "id", Value: "7"}) {
		t.Fatalf("cell update request = %#v", request)
	}
}

func TestCellEditRequestPreservesUnchangedControlCharacters(t *testing.T) {
	m := testModel()
	m.columns = []db.Column{{Name: "id", IsPrimary: true}, {Name: "note"}}
	m.rows = [][]string{{"1", "line one\nline two"}}
	m.cellCursor = 1
	m, _ = m.beginCellEdit()
	request, err := m.cellUpdateRequest()
	if err != nil {
		t.Fatal(err)
	}
	if request.Value != "line one\nline two" {
		t.Fatalf("unchanged editor value = %#v", request.Value)
	}
}

func TestEditCellOnlyAppearsForPrimaryKeyTables(t *testing.T) {
	m := testModel()
	m.focused = true
	if m.helpKeyMap().EditCell.Enabled() {
		t.Fatal("cell editing is enabled without a primary key")
	}
	m.columns = []db.Column{{Name: "id", IsPrimary: true}}
	if !m.helpKeyMap().EditCell.Enabled() {
		t.Fatal("cell editing is disabled with a primary key")
	}
}

func TestEditCellOpensPrefilledEditorAndConfirmation(t *testing.T) {
	m := testModel()
	m.focused = true
	m.columns = []db.Column{{Name: "note", IsPrimary: true}}
	m.rows = [][]string{{"line one\nline two"}}

	m, _ = update(m, keyMsg("e"))
	if m.activeOverlay != overlayCellEdit {
		t.Fatalf("edit overlay = %v", m.activeOverlay)
	}
	if got := m.cellEditInput.Value(); got != `line one\nline two` {
		t.Fatalf("editor value = %q", got)
	}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.activeOverlay != overlayCellEditConfirm {
		t.Fatalf("confirmation overlay = %v", m.activeOverlay)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.activeOverlay != overlayCellEdit {
		t.Fatalf("editor overlay after cancel = %v", m.activeOverlay)
	}
}

func TestConfirmedCellEditUpdatesAndRefreshesCurrentGrid(t *testing.T) {
	client := &fakeDriver{
		cols: []db.Column{{Name: "id", IsPrimary: true}, {Name: "name"}},
		rows: [][]string{{"1", "first"}, {"2", "second"}, {"3", "before"}},
	}
	m := New(Options{Client: client, Timeout: time.Second})
	m.tables = []db.Table{{Schema: "public", Name: "items"}}
	m.focused, m.page, m.pageSize = true, 1, 2
	m.rowCursor, m.cellCursor = 0, 1
	m.columns = append([]db.Column(nil), client.cols...)
	m.rows = [][]string{{"3", "before"}}
	m.visibleColumns = []bool{true, true}
	m.browseFilters = []db.RowFilter{{Column: "name", Operator: db.FilterContains, Value: "before"}}

	m, _ = update(m, keyMsg("e"))
	m.cellEditInput.SetValue("after")
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd = update(m, runCmd(cmd))
	if got := client.lastCellUpdate; got.Column != "name" || got.Value != "after" || len(got.PrimaryKey) != 1 || got.PrimaryKey[0] != (db.PrimaryKeyValue{Column: "id", Value: "3"}) {
		t.Fatalf("cell update request = %#v", got)
	}
	m, _ = update(m, runCmd(cmd))
	if m.status != "updated name" || !m.focused || m.page != 1 || m.rowCursor != 0 || m.cellCursor != 1 || len(m.browseFilters) != 1 || len(m.visibleColumns) != 2 {
		t.Fatalf("refreshed edit state = %#v", m)
	}
}

func TestCellUpdateFailureIsReported(t *testing.T) {
	client := &fakeDriver{cellUpdateErr: errBoom}
	m := New(Options{Client: client, Timeout: time.Second})
	m.tables = []db.Table{{Schema: "public", Name: "items"}}
	m.focused = true
	m.columns = []db.Column{{Name: "id", IsPrimary: true}}
	m.rows = [][]string{{"1"}}
	m, _ = update(m, keyMsg("e"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(m, runCmd(cmd))
	if m.lastErr != errBoom || !strings.Contains(m.status, "cell update failed") {
		t.Fatalf("update failure = status %q, error %v", m.status, m.lastErr)
	}
}

func TestGridNavigationMovesActiveRow(t *testing.T) {
	m := testModel()
	m.rows = makeRows(10)
	m.focused = true

	m, _ = update(m, keyMsg("j"))
	if m.rowCursor != 1 {
		t.Fatalf("j moved to row %d, want 1", m.rowCursor)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.rowCursor != 6 {
		t.Fatalf("Ctrl-D moved to row %d, want 6", m.rowCursor)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.rowCursor != 1 {
		t.Fatalf("Ctrl-U moved to row %d, want 1", m.rowCursor)
	}
	m, _ = update(m, keyMsg("G"))
	if m.rowCursor != 9 {
		t.Fatalf("G moved to row %d, want 9", m.rowCursor)
	}
	m, _ = update(m, keyMsg("g"))
	if m.rowCursor != 0 {
		t.Fatalf("g moved to row %d, want 0", m.rowCursor)
	}
}

func TestGridCountPrefixRepeatsRowAndCellMovement(t *testing.T) {
	m := testModel()
	m.focused = true
	m.rows = makeRows(30)
	m.columns = []db.Column{{Name: "id"}, {Name: "name"}, {Name: "price"}}

	m, _ = update(m, keyMsg("2"))
	m, _ = update(m, keyMsg("3"))
	if m.countPrefix != 23 {
		t.Fatalf("count prefix = %d, want 23", m.countPrefix)
	}
	m, _ = update(m, keyMsg("j"))
	if m.rowCursor != 23 || m.countPrefix != 0 {
		t.Fatalf("23j = row %d, prefix %d; want row 23, prefix 0", m.rowCursor, m.countPrefix)
	}
	m, _ = update(m, keyMsg("2"))
	m, _ = update(m, keyMsg("l"))
	if m.cellCursor != 2 {
		t.Fatalf("2l = column %d, want 2", m.cellCursor)
	}
}

func TestGridCountPrefixCrossesQueryPages(t *testing.T) {
	m := testModel()
	m.focused = true
	m.queryActive = true
	m.pageSize = 2
	m.queryRows = makeRows(5)
	m.setQueryPage()
	m, _ = update(m, keyMsg("3"))
	m, _ = update(m, keyMsg("j"))
	if m.page != 1 || m.rowCursor != 1 {
		t.Fatalf("3j = page %d, row %d; want page 1, row 1", m.page, m.rowCursor)
	}
}

func TestGridSelectionPersistsAcrossFocusAndColumnChanges(t *testing.T) {
	m := testModel()
	m.rows = makeRows(4)
	m.rowCursor = 2
	m.columns = []db.Column{{Name: "id"}, {Name: "name"}}
	m.visibleColumns = []bool{true, true}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.rowCursor != 2 {
		t.Fatalf("row cursor after refocus = %d, want 2", m.rowCursor)
	}
	m.focused = false
	m.activeOverlay = overlayColumnPicker
	m, _ = m.handleColumnsKey(keyMsg(" "))
	if m.rowCursor != 2 {
		t.Fatalf("row cursor after changing visible columns = %d, want 2", m.rowCursor)
	}
}

func TestGridNavigationCrossesPageBoundaries(t *testing.T) {
	m := testModel()
	m.focused = true
	m.queryActive = true
	m.pageSize = 2
	m.queryRows = makeRows(4)
	m.setQueryPage()
	m.rowCursor = 1

	m, _ = update(m, keyMsg("j"))
	if m.page != 1 || m.rowCursor != 0 {
		t.Fatalf("down from final row = page %d, row %d; want page 1, row 0", m.page, m.rowCursor)
	}
	m, _ = update(m, keyMsg("k"))
	if m.page != 0 || m.rowCursor != 1 {
		t.Fatalf("up from first row = page %d, row %d; want page 0, row 1", m.page, m.rowCursor)
	}
}

func TestGridNavigationMovesActiveCellAcrossVisibleColumns(t *testing.T) {
	m := testModel()
	m.focused = true
	m.columns = []db.Column{{Name: "id"}, {Name: "hidden"}, {Name: "name"}}
	m.visibleColumns = []bool{true, false, true}

	m, _ = update(m, keyMsg("l"))
	if m.cellCursor != 1 {
		t.Fatalf("l moved to column %d, want 1", m.cellCursor)
	}
	m, _ = update(m, keyMsg("l"))
	if m.cellCursor != 1 {
		t.Fatalf("cursor moved beyond visible columns: %d", m.cellCursor)
	}
	m, _ = update(m, keyMsg("h"))
	if m.cellCursor != 0 {
		t.Fatalf("h moved to column %d, want 0", m.cellCursor)
	}
}

func TestGridFilterPromptUsesActiveColumnAndCell(t *testing.T) {
	m := testModel()
	m.focused = true
	m.columns = []db.Column{{Name: "id"}, {Name: "name"}}
	m.rows = [][]string{{"1", "Ada"}}
	m.cellCursor = 1

	m, _ = update(m, keyMsg("/"))
	if m.activeOverlay != overlayBrowseFilterOperator {
		t.Fatalf("active overlay = %v, want browse filter operator picker", m.activeOverlay)
	}
	m, _ = m.handleBrowseFilterOperatorKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.activeOverlay != overlayBrowseFilter {
		t.Fatalf("active overlay = %v, want browse filter", m.activeOverlay)
	}
	if got := m.browseFilterInput.Prompt; got != "Filter name = " {
		t.Fatalf("filter prompt = %q", got)
	}
	if got := m.browseFilterInput.Value(); got != "Ada" {
		t.Fatalf("filter value = %q, want Ada", got)
	}
}

func TestGridFilterAppliesEqualityFromPrompt(t *testing.T) {
	m := testModel()
	m.focused = true
	m.columns = []db.Column{{Name: "name"}}
	m.rows = [][]string{{"Ada"}}
	m.pageSize = 2
	m.client = &fakeDriver{cols: m.columns, rows: m.rows, count: 1}

	m, _ = update(m, keyMsg("/"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.browseFilters) != 1 || m.browseFilters[0] != (db.RowFilter{Column: "name", Operator: db.FilterEqual, Value: "Ada"}) {
		t.Fatalf("browse filters = %#v", m.browseFilters)
	}
	if msg := runCmd(cmd); msg != nil {
		m, cmd = update(m, msg)
	}
	if f := m.client.(*fakeDriver); f.lastBrowse.Offset != 0 || f.lastBrowse.Limit != 3 {
		t.Fatalf("browse request = %#v", f.lastBrowse)
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if !strings.Contains(m.status, "of 1") {
		t.Fatalf("status = %q, want filtered row count", m.status)
	}
}

func TestGridFilterStatusAndClearAction(t *testing.T) {
	m := testModel()
	m.focused = true
	m.client = &fakeDriver{cols: []db.Column{{Name: "name"}}, rows: [][]string{{"Ada"}}}
	m.columns = []db.Column{{Name: "name"}}
	m.rows = [][]string{{"Ada"}}
	m.browseFilters = []db.RowFilter{{Column: "name", Operator: db.FilterEqual, Value: "Ada"}}
	m.setRowsStatus(m.tables[0])
	if !strings.Contains(m.status, "filters: name = Ada (x clear)") {
		t.Fatalf("status = %q", m.status)
	}

	m, cmd := update(m, keyMsg("x"))
	if len(m.browseFilters) != 0 || m.page != 0 {
		t.Fatalf("clear filter state = %#v, page %d", m.browseFilters, m.page)
	}
	if msg := runCmd(cmd); msg != nil {
		_, _ = update(m, msg)
	}
}

func TestGridSortCyclesTableOrderAndUsesBrowseRequest(t *testing.T) {
	m := testModel()
	m.focused = true
	m.pageSize = 2
	m.columns = []db.Column{{Name: "name"}}
	m.rows = [][]string{{"Ada"}, {"Zoe"}}
	m.client = &fakeDriver{cols: m.columns, rows: m.rows}

	m, cmd := update(m, keyMsg("S"))
	if m.browseSort != (db.SortSpec{Column: "name"}) || m.page != 0 || cmd == nil {
		t.Fatalf("ascending table sort = %#v, page %d, command %t", m.browseSort, m.page, cmd != nil)
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if got := m.client.(*fakeDriver).lastBrowse.Sort; got != (db.SortSpec{Column: "name"}) {
		t.Fatalf("browse sort = %#v", got)
	}

	m, _ = update(m, keyMsg("S"))
	if got := m.browseSort; got != (db.SortSpec{Column: "name", Descending: true}) {
		t.Fatalf("descending table sort = %#v", got)
	}
	m, _ = update(m, keyMsg("S"))
	if got := m.browseSort; got != (db.SortSpec{}) {
		t.Fatalf("cleared table sort = %#v", got)
	}
}

func TestGridSortOrdersMaterializedQueryResults(t *testing.T) {
	m := testModel()
	m.focused, m.queryActive = true, true
	m.columns = []db.Column{{Name: "name"}}
	m.queryBaseRows = [][]string{{"Zoe"}, {"Ada"}, {"Linus"}}
	m.queryRows = append([][]string(nil), m.queryBaseRows...)
	m.pageSize = 10
	m.setQueryPage()

	m, _ = update(m, keyMsg("S"))
	if got := m.queryRows; got[0][0] != "Ada" || got[2][0] != "Zoe" || m.querySort != (db.SortSpec{Column: "name"}) {
		t.Fatalf("ascending query sort = rows %#v, sort %#v", got, m.querySort)
	}
	m, _ = update(m, keyMsg("S"))
	if got := m.queryRows; got[0][0] != "Zoe" || got[2][0] != "Ada" || !m.querySort.Descending {
		t.Fatalf("descending query sort = rows %#v, sort %#v", got, m.querySort)
	}
	m, _ = update(m, keyMsg("S"))
	if got := m.queryRows; got[0][0] != "Zoe" || got[2][0] != "Linus" || m.querySort != (db.SortSpec{}) {
		t.Fatalf("cleared query sort = rows %#v, sort %#v", got, m.querySort)
	}
}

func TestGridSortReopensStreamedQueryWithSafeDriverSort(t *testing.T) {
	client := &queryStreamFakeDriver{fakeDriver: fakeDriver{rows: [][]string{{"Ada"}, {"Zoe"}}}}
	m := testModel()
	m.client = client
	m.focused, m.queryActive, m.queryStreaming = true, true, true
	m.querySourceSQL, m.querySQL = "select name from people", "select name from people"
	m.columns = []db.Column{{Name: "name"}}
	m.rows = [][]string{{"Ada"}, {"Zoe"}}
	m.pageSize = 2

	m, cmd := update(m, keyMsg("S"))
	if m.querySort != (db.SortSpec{Column: "name"}) || cmd == nil {
		t.Fatalf("stream query sort = %#v, command %t", m.querySort, cmd != nil)
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if len(client.queryRequests) != 1 || client.queryRequests[0].SQL != "select name from people order by name asc" {
		t.Fatalf("stream query requests = %#v", client.queryRequests)
	}
}

func TestGridFilterOperatorPickerSelectsOperator(t *testing.T) {
	m := testModel()
	m.focused = true
	m.columns = []db.Column{{Name: "name"}}
	m.rows = [][]string{{"Ada"}}
	m, _ = update(m, keyMsg("/"))

	m, _ = m.handleBrowseFilterOperatorKey(keyMsg("j"))
	m, _ = m.handleBrowseFilterOperatorKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.browseFilterOperator != db.FilterContains {
		t.Fatalf("operator = %q, want contains", m.browseFilterOperator)
	}
	if got := m.browseFilterInput.Prompt; got != "Filter name contains (case-insensitive) " {
		t.Fatalf("filter prompt = %q", got)
	}
}

func TestGridFilterOperatorPickerAppliesNullFilterImmediately(t *testing.T) {
	m := testModel()
	m.focused = true
	m.client = &fakeDriver{cols: []db.Column{{Name: "deleted_at"}}, rows: [][]string{{"NULL"}}}
	m.columns = []db.Column{{Name: "deleted_at"}}
	m.rows = [][]string{{"NULL"}}
	m.pageSize = 2
	m, _ = update(m, keyMsg("/"))
	for range 8 {
		m, _ = update(m, keyMsg("j"))
	}
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.activeOverlay != overlayNone {
		t.Fatalf("active overlay = %v, want none", m.activeOverlay)
	}
	if len(m.browseFilters) != 1 || m.browseFilters[0] != (db.RowFilter{Column: "deleted_at", Operator: db.FilterIsNull}) {
		t.Fatalf("browse filters = %#v", m.browseFilters)
	}
	if msg := runCmd(cmd); msg != nil {
		_, _ = update(m, msg)
	}
}

func TestBrowseFilterOperatorsIncludeILikeOnlyForPostgres(t *testing.T) {
	m := testModel()
	for _, operator := range m.browseFilterOperators() {
		if operator == db.FilterILike {
			t.Fatal("ILike available without a PostgreSQL driver")
		}
	}
	m.client = &fakeDriver{}
	operators := m.browseFilterOperators()
	if len(operators) != len(standardBrowseFilterOperators)+1 || operators[3] != db.FilterILike {
		t.Fatalf("Postgres operators = %#v", operators)
	}
	m.client = &fakeDriver{dbType: db.DbTypeSQLite}
	for _, operator := range m.browseFilterOperators() {
		if operator == db.FilterILike {
			t.Fatal("ILike available for SQLite")
		}
	}
}

func TestBrowseFilterOperatorsIncludeInclusiveComparisons(t *testing.T) {
	m := testModel()
	operators := m.browseFilterOperators()
	for operator, label := range map[db.FilterOperator]string{
		db.FilterGreaterOrEqual: ">=",
		db.FilterLessOrEqual:    "<=",
	} {
		found := false
		for _, candidate := range operators {
			found = found || candidate == operator
		}
		if !found {
			t.Fatalf("operator %q is missing from picker", operator)
		}
		if got := browseFilterOperatorLabel(operator); got != label {
			t.Fatalf("label for %q = %q, want %q", operator, got, label)
		}
	}
}

func TestBrowseFilterActionsDoNotLeaveSQLMode(t *testing.T) {
	m := testModel()
	m.focused = true
	m.queryActive = true
	m.rows = [][]string{{"query result"}}
	for _, key := range []string{"/", "x"} {
		got, cmd := update(m, keyMsg(key))
		if cmd != nil || !got.queryActive || got.rows[0][0] != "query result" {
			t.Fatalf("%s changed SQL mode: %#v", key, got)
		}
	}
}

func TestNewBrowseContextResetsPagingState(t *testing.T) {
	m := testModel()
	m.browseFilters = []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "1"}}
	m.pendingRowMoves = 4
	m.hasNextPage = true
	m.page = 2
	m, _ = m.clearBrowseFilters()
	if m.pendingRowMoves != 0 || m.hasNextPage || m.page != 0 {
		t.Fatalf("stale browse state = pending %d, next %t, page %d", m.pendingRowMoves, m.hasNextPage, m.page)
	}
}

func TestFilteredStatusDoesNotUseUnfilteredCount(t *testing.T) {
	m := testModel()
	m.rows = [][]string{{"1"}}
	m.browseFilters = []db.RowFilter{{Column: "id", Operator: db.FilterEqual, Value: "1"}}
	m.rowCounts[m.tables[0].String()] = 99
	m.setRowsStatus(m.tables[0])
	if strings.Contains(m.status, "of 99") {
		t.Fatalf("filtered status uses unfiltered count: %q", m.status)
	}
}

func TestGridForeignKeyNavigationRejectsNull(t *testing.T) {
	m := testModel()
	m.schemas = []db.Schema{{Name: "public"}}
	m.focused = true
	m.columns = []db.Column{{Name: "owner_id", ForeignKey: &db.ForeignKey{Schema: "public", Table: "owners", Column: "id"}}}
	m.rows = [][]string{{"NULL"}}
	m, _ = update(m, keyMsg("o"))
	if m.status != "cannot open a NULL reference" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestGridFilterAddsMultipleFilters(t *testing.T) {
	m := testModel()
	m.focused = true
	m.client = &fakeDriver{cols: []db.Column{{Name: "id"}, {Name: "name"}}, rows: [][]string{{"1", "Ada"}}}
	m.columns = []db.Column{{Name: "id"}, {Name: "name"}}
	m.rows = [][]string{{"1", "Ada"}}
	m.pageSize = 2

	m, _ = update(m, keyMsg("/"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	m.cellCursor = 1
	m, _ = update(m, keyMsg("/"))
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.browseFilters) != 2 {
		t.Fatalf("browse filters = %#v, want two filters", m.browseFilters)
	}
	if msg := runCmd(cmd); msg != nil {
		m, _ = update(m, msg)
	}
	if got := m.client.(*fakeDriver).lastBrowse.Filters; len(got) != 2 || got[0].Column != "id" || got[1].Column != "name" {
		t.Fatalf("browse request filters = %#v", got)
	}
	if !strings.Contains(formatBrowseFilters(m.browseFilters), " AND ") {
		t.Fatalf("formatted filters = %q", formatBrowseFilters(m.browseFilters))
	}
}

func TestEnterOpensAndClosesRowDetails(t *testing.T) {
	m := testModel()
	m.focused = true
	m.rows = [][]string{{"1"}}
	m.columns = []db.Column{{Name: "id"}}

	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.activeOverlay != overlayRowDetail {
		t.Fatalf("overlay after Enter = %v, want row details", m.activeOverlay)
	}
	m, _ = update(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.activeOverlay != overlayNone {
		t.Fatalf("overlay after Esc = %v, want none", m.activeOverlay)
	}
}

func TestGridCopyValuesUseVisibleColumns(t *testing.T) {
	m := testModel()
	m.columns = []db.Column{{Name: "id"}, {Name: "secret"}, {Name: "name"}}
	m.rows = [][]string{{"1", "hidden", "books"}}
	m.visibleColumns = []bool{true, false, true}
	m.cellCursor = 1

	if got := m.activeCellValue(); got != "books" {
		t.Fatalf("active cell value = %q, want books", got)
	}
	if got := m.activeRowValue(); got != "1\tbooks" {
		t.Fatalf("active row value = %q, want visible row", got)
	}
}

func TestGridFollowsForeignKeyToReferencedTable(t *testing.T) {
	m := testModel()
	m.schemas = []db.Schema{{Name: "public"}}
	m.tables = []db.Table{{Schema: "public", Name: "categories"}, {Schema: "public", Name: "products"}}
	m.selected = 1
	m.focused = true
	m.columns = []db.Column{{Name: "category_id", ForeignKey: &db.ForeignKey{Schema: "public", Table: "categories", Column: "id"}}}
	m.rows = [][]string{{"1"}}

	m, cmd := update(m, keyMsg("o"))
	if cmd == nil {
		t.Fatal("expected referenced table rows to load")
	}
	if m.selected != 0 || !m.focused {
		t.Fatalf("reference navigation selected %d, focused %t; want 0, true", m.selected, m.focused)
	}
	if len(m.browseFilters) != 1 || m.browseFilters[0] != (db.RowFilter{Column: "id", Operator: db.FilterEqual, Value: "1"}) {
		t.Fatalf("reference browse filters = %#v", m.browseFilters)
	}
	m.setRowsStatus(m.tables[m.selected])
	if !strings.Contains(m.status, "filters: id = 1 (x clear)") {
		t.Fatalf("reference status = %q", m.status)
	}
}

func TestGridForeignKeyNavigationReportsUnavailableTarget(t *testing.T) {
	m := testModel()
	m.schemas = []db.Schema{{Name: "public"}}
	m.focused = true
	m.columns = []db.Column{{Name: "owner_id", ForeignKey: &db.ForeignKey{Schema: "public", Table: "owners", Column: "id"}}}
	m.rows = [][]string{{"1"}}

	m, _ = update(m, keyMsg("o"))
	if m.status != "referenced table is not available" {
		t.Fatalf("reference navigation status = %q", m.status)
	}
}

func TestClipboardResultUpdatesStatus(t *testing.T) {
	m := testModel()
	m, _ = update(m, clipboardWrittenMsg{kind: "cell"})
	if m.status != "copied cell" {
		t.Fatalf("copy status = %q", m.status)
	}
}

func TestCopiedStatusReturnsToBrowseStatus(t *testing.T) {
	m := testModel()
	m.columns = []db.Column{{Name: "id"}}
	m.rows = [][]string{{"1"}}
	m.rowCounts[m.tables[0].String()] = 1
	m.copyStatusID = 1

	m, _ = update(m, clipboardWrittenMsg{kind: "cell", copyStatusID: 1})
	m, _ = update(m, copyStatusClearedMsg{kind: "cell", copyStatusID: 1})
	if got, want := m.status, "public.a page 1 (1 rows of 1)"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestOlderCopiedStatusDoesNotOverrideNewStatus(t *testing.T) {
	m := testModel()
	m.copyStatusID = 2
	m.status = "loading public.a page 1..."

	m, _ = update(m, copyStatusClearedMsg{kind: "cell", copyStatusID: 1})
	if got, want := m.status, "loading public.a page 1..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestRowDetailsScroll(t *testing.T) {
	m := testModel()
	m.width = 40
	m.height = 5
	m.activeOverlay = overlayRowDetail
	m.rows = [][]string{{"1", "2", "3"}}
	m.columns = []db.Column{{Name: "id"}, {Name: "parent_id"}, {Name: "name"}}

	m, _ = update(m, keyMsg("j"))
	if m.detailScroll != 1 {
		t.Fatalf("detail scroll = %d, want 1", m.detailScroll)
	}
}

func TestHelpOverlay(t *testing.T) {
	m := testModel()
	m, _ = update(m, keyMsg("?"))
	if m.activeOverlay != overlayHelp {
		t.Fatal("expected help overlay to open")
	}
	if got := m.View(); !strings.Contains(got, "Keyboard shortcuts") {
		t.Fatalf("help overlay missing heading: %q", got)
	}
	m, _ = update(m, keyMsg("x"))
	if m.activeOverlay != overlayNone {
		t.Fatal("expected any key to close help overlay")
	}
}

func TestColumnPickerTogglesColumnsButKeepsOneVisible(t *testing.T) {
	m := testModel()
	m.columns = []db.Column{{Name: "id"}, {Name: "email"}}
	m.visibleColumns = []bool{true, true}
	m.activeOverlay = overlayColumnPicker

	m, _ = m.handleColumnsKey(keyMsg(" "))
	if m.visibleColumns[0] {
		t.Fatal("expected first column to be hidden")
	}
	m.columnCursor = 1
	m, _ = m.handleColumnsKey(keyMsg(" "))
	if !m.visibleColumns[1] {
		t.Fatal("expected final visible column to remain selected")
	}
	if !strings.Contains(m.status, "at least one column") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestStaleRowsAreIgnored(t *testing.T) {
	m := testModel()
	m.loadID = 2
	m.rows = [][]string{{"current"}}
	got, _ := m.handleRowsLoaded(rowsLoadedMsg{requestID: 1, rows: [][]string{{"stale"}}})
	if got.rows[0][0] != "current" {
		t.Fatalf("stale rows replaced current rows: %#v", got.rows)
	}
}

func TestSchemaCancelKeepsCommittedSchema(t *testing.T) {
	m := testModel()
	m.schemas = []db.Schema{{Name: "public"}, {Name: "analytics"}}
	m.schema = 0
	m.schemaCursor = 0
	m.activeOverlay = overlaySchemaPicker
	m, _ = m.handleSchemaKey(keyMsg("j"))
	m, _ = m.handleSchemaKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.schema != 0 || m.currentSchema() != "public" {
		t.Fatalf("schema changed after cancel: %q", m.currentSchema())
	}
}

func TestColumnVisibilityResetsForDifferentTable(t *testing.T) {
	m := testModel()
	m.columns = []db.Column{{Name: "id"}, {Name: "secret"}}
	m.ensureVisibleColumns("public.accounts")
	m.visibleColumns[1] = false
	m.ensureVisibleColumns("public.events")
	if !m.visibleColumns[0] || !m.visibleColumns[1] {
		t.Fatalf("visibility leaked to different table: %#v", m.visibleColumns)
	}
}

func TestFilterCancelRestoresPreviousFilter(t *testing.T) {
	m := testModel()
	m.activeOverlay = overlayFilter
	m.filterPrevious = "accounts"
	m.filterInput.SetValue("events")
	m, _ = m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEsc})
	if got := m.filterInput.Value(); got != "accounts" {
		t.Fatalf("filter after cancel = %q", got)
	}
}

func TestHandlersReportErrors(t *testing.T) {
	m := testModel()
	got, _ := m.handleTablesLoaded(tablesLoadedMsg{err: errBoom})
	if got.status != "failed to load tables: boom" {
		t.Fatalf("unexpected status: %q", got.status)
	}
	got, _ = m.handleRowsLoaded(rowsLoadedMsg{err: errBoom})
	if got.status != "rows error: boom" {
		t.Fatalf("unexpected status: %q", got.status)
	}
}

func TestKeepInView(t *testing.T) {
	if got := keepInView(2, 0, 4, 7); got != 0 {
		t.Fatalf("selected within window, want offset 0, got %d", got)
	}
	if got := keepInView(4, 0, 4, 7); got != 1 {
		t.Fatalf("selection moved past bottom edge, want offset 1, got %d", got)
	}
	if got := keepInView(6, 3, 4, 7); got != 3 {
		t.Fatalf("selection at end, want offset 3, got %d", got)
	}
	if got := keepInView(1, 3, 4, 7); got != 1 {
		t.Fatalf("selection moved above window, want offset 1, got %d", got)
	}
	if got := keepInView(3, 0, 4, 3); got != 0 {
		t.Fatalf("list shorter than window, want offset 0, got %d", got)
	}
}

func TestScrollWithMoveSelection(t *testing.T) {
	m := testModel()
	m.tables = make([]db.Table, 7)
	for i := range m.tables {
		m.tables[i] = db.Table{Schema: "public", Name: "t"}
	}
	m.selected = 3
	for i := 0; i < 3; i++ {
		m, _ = m.moveSelection(+1)
	}
	if m.selected != 6 {
		t.Fatalf("want selected 6, got %d", m.selected)
	}
	if m.scroll != 3 {
		t.Fatalf("want scroll 3 at bottom edge, got %d", m.scroll)
	}
	for i := 0; i < 6; i++ {
		m, _ = m.moveSelection(-1)
	}
	if m.selected != 0 {
		t.Fatalf("want selected 0, got %d", m.selected)
	}
	if m.scroll != 0 {
		t.Fatalf("want scroll 0 at top, got %d", m.scroll)
	}
}

func TestComputedPageSize(t *testing.T) {
	m := testModel()
	m.height = 40
	if got := m.computedPageSize(); got != 32 {
		t.Fatalf("height 40: want page size 32, got %d", got)
	}
	m.height = 120
	if got := m.computedPageSize(); got != maxPageSize {
		t.Fatalf("tall screen: want page size %d, got %d", maxPageSize, got)
	}
	m.height = 4
	if got := m.computedPageSize(); got != 1 {
		t.Fatalf("tiny screen: want page size 1, got %d", got)
	}
}

func TestWindowSizeUpdatesPageSize(t *testing.T) {
	m := testModel()
	m.height = 0
	m.pageSize = maxPageSize

	m, _ = m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.pageSize != 32 {
		t.Fatalf("want page size 32, got %d", m.pageSize)
	}
}

func TestWindowSizeReloadsRows(t *testing.T) {
	m := testModel()
	m.height = 40
	m, _ = m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.pageSize != 32 {
		t.Fatalf("want page size 32, got %d", m.pageSize)
	}
	if _, cmd := m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40}); cmd != nil {
		t.Fatal("expected no reload when page size is unchanged")
	}
	m.pageSize = 0
	if _, cmd := m.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 40}); cmd == nil {
		t.Fatal("expected reload when page size changes")
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }

var errBoom error = testErr("boom")
