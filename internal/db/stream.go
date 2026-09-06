package db

import (
	"context"
	"fmt"
	"io"
)

// RowStream reads formatted database rows in their query order. Callers must
// close a successfully opened stream, even after Next returns io.EOF. The
// context supplied when opening a stream remains responsible for cancellation.
type RowStream interface {
	// Next returns the next row, or io.EOF after the final row.
	Next() ([]string, error)
	// Close releases any query and connection resources held by the stream.
	Close() error
}

// TableRowStreamRequest describes a structured, unpaginated table browse
// stream. Filters have the same parameterized semantics as BrowseRequest.
type TableRowStreamRequest struct {
	Table   Table
	Filters []RowFilter
}

// TableRowStreamer is implemented by drivers that can keep a table browse
// query open and deliver its rows incrementally. The supplied context must stay
// active until the returned stream is closed.
type TableRowStreamer interface {
	OpenTableRowStream(ctx context.Context, request TableRowStreamRequest) ([]Column, RowStream, error)
}

// ReadRowStream reads at most limit rows from stream. exhausted is true only
// when the stream reached io.EOF while reading this batch. The caller retains
// ownership of stream and must close it when no further rows are needed.
func ReadRowStream(stream RowStream, limit int) (rows [][]string, exhausted bool, err error) {
	if limit < 1 {
		return nil, false, fmt.Errorf("row stream limit must be positive")
	}
	for len(rows) < limit {
		row, err := stream.Next()
		if err == io.EOF {
			return rows, true, nil
		}
		if err != nil {
			return rows, false, err
		}
		rows = append(rows, row)
	}
	return rows, false, nil
}
