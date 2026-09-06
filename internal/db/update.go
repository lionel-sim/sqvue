package db

import (
	"context"
	"errors"
)

// PrimaryKeyValue identifies one declared primary-key column in a row.
type PrimaryKeyValue struct {
	Column string
	Value  string
}

// CellUpdateRequest describes one parameterized table-cell update.
type CellUpdateRequest struct {
	Table      Table
	Column     string
	Value      string
	PrimaryKey []PrimaryKeyValue
}

// Validate ensures the request can identify exactly one table row.
func (r CellUpdateRequest) Validate() error {
	if r.Table.Name == "" {
		return errors.New("update table is required")
	}
	if r.Column == "" {
		return errors.New("update column is required")
	}
	if len(r.PrimaryKey) == 0 {
		return errors.New("update requires primary-key values")
	}
	for _, value := range r.PrimaryKey {
		if value.Column == "" {
			return errors.New("primary-key column is required")
		}
	}
	return nil
}

// CellUpdater is implemented by drivers that can update one primary-key row.
type CellUpdater interface {
	UpdateCell(context.Context, CellUpdateRequest) error
}
