package db

import "testing"

func TestRowInsertRequestValidate(t *testing.T) {
	valid := RowInsertRequest{
		Table: Table{Schema: "public", Name: "items"},
		Values: []RowInsertValue{
			{Column: "name", Kind: RowInsertLiteral, Value: "widget"},
			{Column: "archived_at", Kind: RowInsertNull},
			{Column: "created_at", Kind: RowInsertCurrentTimestamp},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request error = %v", err)
	}
	if err := (RowInsertRequest{Table: valid.Table}).Validate(); err != nil {
		t.Fatalf("default-only request error = %v", err)
	}
	for _, request := range []RowInsertRequest{
		{},
		{Table: valid.Table, Values: []RowInsertValue{{Kind: RowInsertLiteral, Value: "widget"}}},
		{Table: valid.Table, Values: []RowInsertValue{{Column: "name", Kind: RowInsertLiteral}}},
		{Table: valid.Table, Values: []RowInsertValue{{Column: "name", Kind: RowInsertNull, Value: "not nil"}}},
		{Table: valid.Table, Values: []RowInsertValue{{Column: "name", Kind: RowInsertCurrentTimestamp, Value: "not nil"}}},
		{Table: valid.Table, Values: []RowInsertValue{{Column: "name", Kind: RowInsertLiteral, Value: "one"}, {Column: "name", Kind: RowInsertLiteral, Value: "two"}}},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("Validate() accepted %#v", request)
		}
	}
}
