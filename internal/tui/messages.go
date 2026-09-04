package tui

import "sqvue/internal/db"

type (
	schemasLoadedMsg struct {
		schemas []db.Schema
		err     error
	}
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
	countLoadedMsg struct {
		table db.Table
		count int64
		err   error
	}
	queryLoadedMsg struct {
		result db.Result
		err    error
	}
)
