package config

import (
	"os"
	"path/filepath"
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

func TestLoadOrCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "config.toml")

	file, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if !created {
		t.Fatal("LoadOrCreate() did not report a newly-created config")
	}
	if file.Settings.DefaultProfile != "" || len(file.Connections) != 0 {
		t.Fatalf("new config = %#v, want empty configuration", file)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(contents), "[connections.local]") {
		t.Fatalf("default config missing example profile:\n%s", contents)
	}
	if !strings.Contains(string(contents), "theme = \"default\"") {
		t.Fatalf("default config missing theme setting:\n%s", contents)
	}

	_, created, err = LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate() error = %v", err)
	}
	if created {
		t.Fatal("LoadOrCreate() reported an existing config as newly created")
	}
}

func TestLoadOrCreateLoadsProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	contents := `[settings]
default_profile = "work"
export_directory = "./exports"
theme = "high-contrast"

[connections.work]
host = "db.example.com"
port = 5433
user = "reader"
database = "analytics"
sslmode = "verify-full"
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	file, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if created {
		t.Fatal("LoadOrCreate() reported an existing config as newly created")
	}
	if file.Settings.DefaultProfile != "work" {
		t.Fatalf("default profile = %q, want work", file.Settings.DefaultProfile)
	}
	if file.Settings.ExportDirectory != "./exports" {
		t.Fatalf("export directory = %q, want ./exports", file.Settings.ExportDirectory)
	}
	if file.Settings.Theme != "high-contrast" {
		t.Fatalf("theme = %q, want high-contrast", file.Settings.Theme)
	}
	profile, err := file.Profile("work")
	if err != nil {
		t.Fatalf("Profile() error = %v", err)
	}
	if profile.Host != "db.example.com" || profile.Port != 5433 || profile.SSLMode != "verify-full" {
		t.Fatalf("Profile() = %#v", profile)
	}
}

func TestSettingsResolveThemeDefaultsAndValidates(t *testing.T) {
	if _, err := (Settings{}).ResolveTheme(); err != nil {
		t.Fatalf("default theme error = %v", err)
	}
	if _, err := (Settings{Theme: "light"}).ResolveTheme(); err != nil {
		t.Fatalf("light theme error = %v", err)
	}
	if _, err := (Settings{Theme: "sepia"}).ResolveTheme(); err == nil || !strings.Contains(err.Error(), "unknown theme") {
		t.Fatalf("invalid theme error = %v, want unknown theme", err)
	}
}

func TestLoadOrCreateRejectsInvalidTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[settings]\ntheme = \"sepia\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, _, err := LoadOrCreate(path); err == nil || !strings.Contains(err.Error(), "settings.theme") {
		t.Fatalf("LoadOrCreate() error = %v, want theme validation error", err)
	}
}

func TestDBProfileMerge(t *testing.T) {
	base := DefaultDBProfile()
	got := base.Merge(DBProfile{DBType: "sqlite", Host: "db.example.com", Database: "analytics", SSLMode: "verify-full"})
	if got.DBType != "sqlite" || got.Host != "db.example.com" || got.Port != 5432 || got.User != "postgres" || got.Database != "analytics" || got.SSLMode != "verify-full" {
		t.Fatalf("Merge() = %#v", got)
	}
}

func TestDBProfileValidateForSQLite(t *testing.T) {
	if err := (DBProfile{ConnString: "sqvue.db", Timeout: time.Second}).ValidateFor("sqlite"); err != nil {
		t.Fatalf("ValidateFor(sqlite) error = %v", err)
	}
	if err := (DBProfile{Timeout: time.Second}).ValidateFor("sqlite"); err == nil {
		t.Fatal("ValidateFor(sqlite) accepted a missing database path")
	}
}

func TestDBProfileValidateForMySQL(t *testing.T) {
	if err := (DBProfile{ConnString: "user@tcp(localhost:3306)/app", Timeout: time.Second}).ValidateFor("mysql"); err != nil {
		t.Fatalf("ValidateFor(mysql) error = %v", err)
	}
	if err := (DBProfile{Timeout: time.Second}).ValidateFor("mysql"); err == nil {
		t.Fatal("ValidateFor(mysql) accepted a missing connection string")
	}
}

func TestQueryStorePersistsPerProfileHistoryAndQueries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queries.toml")
	store, err := LoadQueryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddHistory("work", "select 1\nfrom accounts"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddHistory("work", "select 1\nfrom accounts"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddHistory("other", "select 2"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveQuery("work", "daily accounts", "select *\nfrom accounts"); err != nil {
		t.Fatal(err)
	}
	if err := store.RenameQuery("work", "daily accounts", "accounts daily"); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadQueryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.History("work"); len(got) != 1 || got[0] != "select 1\nfrom accounts" {
		t.Fatalf("work history = %#v", got)
	}
	if got := loaded.History("other"); len(got) != 1 || got[0] != "select 2" {
		t.Fatalf("other history = %#v", got)
	}
	if got := loaded.QueryNames("work"); len(got) != 1 || got[0] != "accounts daily" {
		t.Fatalf("query names = %#v", got)
	}
	if got, ok := loaded.Query("work", "accounts daily"); !ok || got != "select *\nfrom accounts" {
		t.Fatalf("saved query = %q, present %t", got, ok)
	}
	if err := loaded.DeleteQuery("work", "accounts daily"); err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Query("work", "accounts daily"); ok {
		t.Fatal("deleted query remained available")
	}
}

func TestQueryStorePathUsesConfigDirectory(t *testing.T) {
	if got := QueryStorePath("/tmp/sqvue/config.toml"); got != "/tmp/sqvue/queries.toml" {
		t.Fatalf("QueryStorePath() = %q", got)
	}
}
