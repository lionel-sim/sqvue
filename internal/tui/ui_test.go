package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sqvue/internal/db"
)

func testModel() Model {
	m := New(Options{Client: nil, Timeout: 0})
	m.tables = []db.Table{{Schema: "public", Name: "a"}, {Schema: "public", Name: "b"}}
	m.loading = false
	return m
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
