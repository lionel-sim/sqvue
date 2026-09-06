package db

import (
	"context"
	"errors"
)

// RowInsertValueKind identifies how a form field should be written.
// Default values are represented by omitting the column from RowInsertRequest.
type RowInsertValueKind string

const (
	// RowInsertLiteral writes Value as a parameterized database value.
	RowInsertLiteral RowInsertValueKind = "literal"
	// RowInsertNull writes a database NULL. Value must be nil.
	RowInsertNull RowInsertValueKind = "null"
	// RowInsertCurrentTimestamp writes the database's current timestamp.
	// It is intentionally a typed value rather than user-supplied SQL.
	RowInsertCurrentTimestamp RowInsertValueKind = "current_timestamp"
)

// RowInsertValue describes one supplied value for a row insert.
type RowInsertValue struct {
	Column string
	Kind   RowInsertValueKind
	Value  any
}

// RowInsertRequest describes one parameterized insert into a base table.
// An empty Values slice asks the database to use defaults for every column.
type RowInsertRequest struct {
	Table  Table
	Values []RowInsertValue
}

// Validate checks that the request can safely describe a single row insert.
func (r RowInsertRequest) Validate() error {
	if r.Table.Name == "" {
		return errors.New("insert table is required")
	}
	seen := make(map[string]struct{}, len(r.Values))
	for _, value := range r.Values {
		if value.Column == "" {
			return errors.New("insert column is required")
		}
		if _, ok := seen[value.Column]; ok {
			return errors.New("insert columns must be unique")
		}
		seen[value.Column] = struct{}{}
		switch value.Kind {
		case RowInsertLiteral:
			if value.Value == nil {
				return errors.New("literal insert value is required")
			}
		case RowInsertNull:
			if value.Value != nil {
				return errors.New("null insert value must be nil")
			}
		case RowInsertCurrentTimestamp:
			if value.Value != nil {
				return errors.New("current timestamp insert value must be nil")
			}
		default:
			return errors.New("insert value kind is required")
		}
	}
	return nil
}

// RowInserter is implemented by drivers that can insert a single table row.
type RowInserter interface {
	InsertRow(context.Context, RowInsertRequest) error
}
