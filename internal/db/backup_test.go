package db

import "testing"

func TestBackupRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		request BackupRequest
		wantErr bool
	}{
		{
			name:    "database",
			request: BackupRequest{Scope: BackupScopeDatabase, Path: "backup.sql"},
		},
		{
			name:    "schema",
			request: BackupRequest{Scope: BackupScopeSchema, Path: "backup.sql", Schema: "public"},
		},
		{
			name:    "table",
			request: BackupRequest{Scope: BackupScopeTable, Path: "backup.sql", Table: Table{Schema: "public", Name: "orders"}},
		},
		{
			name:    "missing path",
			request: BackupRequest{Scope: BackupScopeDatabase},
			wantErr: true,
		},
		{
			name:    "missing schema target",
			request: BackupRequest{Scope: BackupScopeSchema, Path: "backup.sql"},
			wantErr: true,
		},
		{
			name:    "missing table target",
			request: BackupRequest{Scope: BackupScopeTable, Path: "backup.sql", Table: Table{Schema: "public"}},
			wantErr: true,
		},
		{
			name:    "unknown scope",
			request: BackupRequest{Scope: "rows", Path: "backup.sql"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.request.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
