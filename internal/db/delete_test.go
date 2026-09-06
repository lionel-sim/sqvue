package db

import "testing"

func TestRowDeleteRequestValidate(t *testing.T) {
	valid := RowDeleteRequest{
		Table:      Table{Schema: "public", Name: "items"},
		PrimaryKey: []PrimaryKeyValue{{Column: "tenant", Value: "north"}, {Column: "id", Value: "7"}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request error = %v", err)
	}
	for _, request := range []RowDeleteRequest{
		{},
		{Table: valid.Table},
		{Table: valid.Table, PrimaryKey: []PrimaryKeyValue{{Value: "7"}}},
		{Table: valid.Table, PrimaryKey: []PrimaryKeyValue{{Column: "id", Value: "7"}, {Column: "id", Value: "8"}}},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("Validate() accepted %#v", request)
		}
	}
}
