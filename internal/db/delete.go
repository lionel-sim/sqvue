package db

import (
	"context"
	"errors"
)

// RowDeleteRequest describes deletion of one row identified by its declared
// primary-key values.
type RowDeleteRequest struct {
	Table      Table
	PrimaryKey []PrimaryKeyValue
}

// Validate ensures the request can identify exactly one table row.
func (r RowDeleteRequest) Validate() error {
	if r.Table.Name == "" {
		return errors.New("delete table is required")
	}
	if len(r.PrimaryKey) == 0 {
		return errors.New("delete requires primary-key values")
	}
	seen := make(map[string]struct{}, len(r.PrimaryKey))
	for _, value := range r.PrimaryKey {
		if value.Column == "" {
			return errors.New("primary-key column is required")
		}
		if _, ok := seen[value.Column]; ok {
			return errors.New("delete primary-key columns must be unique")
		}
		seen[value.Column] = struct{}{}
	}
	return nil
}

// RowDeleter is implemented by drivers that can delete one primary-key row.
type RowDeleter interface {
	DeleteRow(context.Context, RowDeleteRequest) error
}
