package db

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
)

type testRowStream struct {
	rows   [][]string
	err    error
	index  int
	closed bool
}

func (s *testRowStream) Next() ([]string, error) {
	if s.index < len(s.rows) {
		row := s.rows[s.index]
		s.index++
		return row, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, io.EOF
}

func (s *testRowStream) Close() error {
	s.closed = true
	return nil
}

type testTableRowStreamer struct{}

func (testTableRowStreamer) OpenTableRowStream(context.Context, TableRowStreamRequest) ([]Column, RowStream, error) {
	return nil, &testRowStream{}, nil
}

var _ RowStream = (*testRowStream)(nil)
var _ TableRowStreamer = testTableRowStreamer{}

func TestReadRowStreamReturnsBatchesAndEOF(t *testing.T) {
	stream := &testRowStream{rows: [][]string{{"1"}, {"2"}, {"3"}}}

	rows, exhausted, err := ReadRowStream(stream, 2)
	if err != nil {
		t.Fatalf("ReadRowStream() error = %v", err)
	}
	if exhausted {
		t.Fatal("first batch reported exhausted")
	}
	if want := [][]string{{"1"}, {"2"}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("first batch = %#v, want %#v", rows, want)
	}

	rows, exhausted, err = ReadRowStream(stream, 2)
	if err != nil {
		t.Fatalf("ReadRowStream() error = %v", err)
	}
	if !exhausted {
		t.Fatal("last batch did not report exhaustion")
	}
	if want := [][]string{{"3"}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("last batch = %#v, want %#v", rows, want)
	}
}

func TestReadRowStreamReturnsPartialRowsBeforeError(t *testing.T) {
	wantErr := errors.New("stream failed")
	stream := &testRowStream{rows: [][]string{{"1"}}, err: wantErr}

	rows, exhausted, err := ReadRowStream(stream, 2)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ReadRowStream() error = %v, want %v", err, wantErr)
	}
	if exhausted {
		t.Fatal("errored stream reported exhausted")
	}
	if want := [][]string{{"1"}}; !reflect.DeepEqual(rows, want) {
		t.Fatalf("partial rows = %#v, want %#v", rows, want)
	}
}

func TestReadRowStreamRejectsNonPositiveLimit(t *testing.T) {
	if _, _, err := ReadRowStream(&testRowStream{}, 0); err == nil {
		t.Fatal("ReadRowStream() succeeded with a zero limit")
	}
}
