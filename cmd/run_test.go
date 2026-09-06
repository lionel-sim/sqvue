package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"sqvue/internal/config"
)

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "DATABASE_URL", "fallback"); got != "DATABASE_URL" {
		t.Fatalf("firstNonEmpty() = %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty() = %q", got)
	}
}

func TestExportDirectory(t *testing.T) {
	if got := exportDirectory("relative/exports"); !filepath.IsAbs(got) {
		t.Fatalf("exportDirectory() = %q, want absolute path", got)
	}
	if got := exportDirectory(""); !filepath.IsAbs(got) {
		t.Fatalf("exportDirectory(\"\") = %q, want current working directory", got)
	}
}

func TestTUIProfilesAreSortedAndUseConfiguredConnections(t *testing.T) {
	profiles := tuiProfiles(config.File{Connections: map[string]config.DBProfile{
		"zeta":  {DBType: "sqlite", ConnString: "zeta.db"},
		"alpha": {DBType: "mysql", ConnString: "reader@tcp(localhost:3306)/app"},
	}})
	if len(profiles) != 2 || profiles[0].Name != "alpha" || profiles[1].Name != "zeta" {
		t.Fatalf("tuiProfiles() = %#v", profiles)
	}
	if profiles[0].Config.DbType != "mysql" || profiles[1].Config.DSN != "zeta.db" || profiles[0].Timeout <= 0 {
		t.Fatalf("tui profiles = %#v", profiles)
	}
}

func TestLoadProfileUsesConfiguredDefault(t *testing.T) {
	originalProfileFlag := profileFlag
	originalConnFlag := connFlag
	originalHostFlag := hostFlag
	originalPortFlag := portFlag
	originalUserFlag := userFlag
	originalPasswordFlag := passwordFlag
	originalDatabaseFlag := databaseFlag
	originalSSLModeFlag := sslModeFlag
	originalDBTypeFlag := dbTypeFlag
	defer func() {
		profileFlag = originalProfileFlag
		connFlag = originalConnFlag
		hostFlag = originalHostFlag
		portFlag = originalPortFlag
		userFlag = originalUserFlag
		passwordFlag = originalPasswordFlag
		databaseFlag = originalDatabaseFlag
		sslModeFlag = originalSSLModeFlag
		dbTypeFlag = originalDBTypeFlag
	}()

	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("DATABASE_URL", "")
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	contents := `[settings]
default_profile = "staging"

[connections.staging]
host = "staging.example.com"
port = 5433
user = "viewer"
database = "app"
sslmode = "verify-full"

[connections.work]
conn = "postgres://work.example.com/work"

[connections.local_sqlite]
db_type = "sqlite"
conn = "sqvue.db"
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	profileFlag = ""
	connFlag = ""
	hostFlag = "localhost"
	portFlag = 5432
	userFlag = "postgres"
	passwordFlag = ""
	databaseFlag = ""
	sslModeFlag = "require"

	profile, created, gotPath, err := loadProfile(rootCmd)
	if err != nil {
		t.Fatalf("loadProfile() error = %v", err)
	}
	if created {
		t.Fatal("loadProfile() created an existing config")
	}
	if gotPath != path {
		t.Fatalf("config path = %q, want %q", gotPath, path)
	}
	if profile.Host != "staging.example.com" || profile.Port != 5433 || profile.User != "viewer" || profile.Database != "app" || profile.SSLMode != "verify-full" {
		t.Fatalf("profile = %#v", profile)
	}

	t.Setenv("DATABASE_URL", "postgres://environment.example.com/env")
	profile, _, _, err = loadProfile(rootCmd)
	if err != nil {
		t.Fatalf("loadProfile() with DATABASE_URL error = %v", err)
	}
	if profile.ConnString != "postgres://environment.example.com/env" {
		t.Fatalf("environment connection string = %q", profile.ConnString)
	}

	profileFlag = "work"
	profile, _, _, err = loadProfile(rootCmd)
	if err != nil {
		t.Fatalf("loadProfile() with --profile error = %v", err)
	}
	if profile.ConnString != "postgres://work.example.com/work" {
		t.Fatalf("explicit profile connection string = %q", profile.ConnString)
	}

	connFlag = "postgres://flag.example.com/flag"
	profile, _, _, err = loadProfile(rootCmd)
	if err != nil {
		t.Fatalf("loadProfile() with --conn error = %v", err)
	}
	if profile.ConnString != "postgres://flag.example.com/flag" {
		t.Fatalf("flag connection string = %q", profile.ConnString)
	}

	connFlag = ""
	profileFlag = "local_sqlite"
	profile, _, _, err = loadProfile(rootCmd)
	if err != nil {
		t.Fatalf("loadProfile() with SQLite profile error = %v", err)
	}
	if profile.DBType != "sqlite" || profile.ConnString != "sqvue.db" {
		t.Fatalf("SQLite profile = %#v", profile)
	}
}
