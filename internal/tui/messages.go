package tui

import "sqvue/internal/db"

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
	descriptionsLoadedMsg struct {
		info db.TableInfo
		err  error
	}
)
