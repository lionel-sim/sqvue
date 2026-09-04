package tui

import "sqvue/internal/db"

type (
	schemasLoadedMsg struct {
		requestID uint64
		schemas   []db.Schema
		err       error
	}
	tablesLoadedMsg struct {
		requestID uint64
		tables    []db.Table
		err       error
	}
	rowsLoadedMsg struct {
		requestID uint64
		columns   []db.Column
		rows      [][]string
		err       error
	}
	descriptionsLoadedMsg struct {
		requestID uint64
		info      db.TableInfo
		err       error
	}
	countLoadedMsg struct {
		requestID uint64
		table     db.Table
		count     int64
		err       error
	}
	queryLoadedMsg struct {
		requestID uint64
		result    db.Result
		err       error
	}
)
