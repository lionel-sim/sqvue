package db

import "testing"

func TestCellUpdateRequestValidate(t *testing.T) {
	valid := CellUpdateRequest{
		Table:      Table{Schema: "public", Name: "items"},
		Column:     "name",
		Value:      "updated",
		PrimaryKey: []PrimaryKeyValue{{Column: "id", Value: "1"}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request error = %v", err)
	}
	for _, request := range []CellUpdateRequest{
		{},
		{Table: valid.Table},
		{Table: valid.Table, Column: valid.Column},
		{Table: valid.Table, Column: valid.Column, PrimaryKey: []PrimaryKeyValue{{Value: "1"}}},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("Validate() accepted %#v", request)
		}
	}
}
