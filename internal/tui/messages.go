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
	tableStreamRowsLoadedMsg struct {
		requestID uint64
		columns   []db.Column
		rows      [][]string
		exhausted bool
		rowCount  *int64
		browseKey string
		stream    db.RowStream
		cancel    func()
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
		browseKey string
		err       error
	}
	queryLoadedMsg struct {
		requestID uint64
		sql       string
		result    db.Result
		err       error
	}
	queryStreamRowsLoadedMsg struct {
		requestID uint64
		sql       string
		initial   bool
		columns   []string
		rows      [][]string
		exhausted bool
		stream    db.QueryRowStream
		cancel    func()
		duration  int64
		affected  int64
		err       error
	}
	clipboardWrittenMsg struct {
		kind         string
		copyStatusID uint64
		err          error
	}
	copyStatusClearedMsg struct {
		kind         string
		copyStatusID uint64
	}
	exportedMsg struct {
		exportID uint64
		format   exportFormat
		path     string
		rows     int
		err      error
	}
	backupCompletedMsg struct {
		backupID uint64
		path     string
		err      error
	}
	cellUpdatedMsg struct {
		updateID uint64
		column   string
		err      error
	}
	rowInsertedMsg struct {
		insertID uint64
		err      error
	}
	profileConnectedMsg struct {
		reconnectID uint64
		profile     ConnectionProfile
		client      db.Driver
		err         error
	}
)
