package config

import (
	"strings"
	"testing"
	"time"
)

func TestDBProfileValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile DBProfile
		wantErr string
	}{
		{
			name:    "connection string",
			profile: DBProfile{ConnString: "postgres://localhost/db", Timeout: time.Second},
		},
		{
			name:    "connection fields",
			profile: DBProfile{Host: "localhost", Port: 5432, User: "postgres", Database: "sqvue", Timeout: time.Second},
		},
		{
			name:    "missing database",
			profile: DBProfile{Host: "localhost", Port: 5432, User: "postgres", Timeout: time.Second},
			wantErr: "provide --conn",
		},
		{
			name:    "invalid port",
			profile: DBProfile{Host: "localhost", Port: 0, User: "postgres", Database: "sqvue", Timeout: time.Second},
			wantErr: "port must be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
