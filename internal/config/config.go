package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"sqvue/internal/theme"
)

type DBProfile struct {
	DBType             string        `toml:"db_type"`
	ConnString         string        `toml:"conn"`
	Host               string        `toml:"host"`
	Port               int           `toml:"port"`
	User               string        `toml:"user"`
	Password           string        `toml:"password"`
	Database           string        `toml:"database"`
	SSLMode            string        `toml:"sslmode"`
	RetainQueryHistory bool          `toml:"retain_query_history"`
	Timeout            time.Duration `toml:"-"`
}

// File is sqvue's on-disk configuration. Connections are keyed by profile name.
type File struct {
	Settings    Settings             `toml:"settings"`
	Connections map[string]DBProfile `toml:"connections"`
}

type Settings struct {
	DefaultProfile  string `toml:"default_profile"`
	ExportDirectory string `toml:"export_directory"`
	Theme           string `toml:"theme"`
}

// QueryStore keeps the SQL workflow data separate from connection credentials.
// It lives beside config.toml in queries.toml and is keyed by connection profile.
type QueryStore struct {
	Profiles map[string]QueryProfile `toml:"profiles"`
	path     string
}

type QueryProfile struct {
	History []string          `toml:"history"`
	Queries map[string]string `toml:"queries"`
}

const queryHistoryLimit = 100

const defaultFile = `# sqvue configuration
#
# Define named PostgreSQL connections here, then select one with
# settings.default_profile or --profile. Do not commit passwords to source control.

[settings]
# default_profile = "local"
# export_directory = "." # Defaults to sqvue's current working directory.
# theme = "default" # Available: default, light, high-contrast.

# [connections.local]
# db_type = "postgres"
# host = "localhost"
# port = 5432
# user = "postgres"
# database = "my_database"
# sslmode = "disable"
# retain_query_history = true # Off by default; stores executed SQL in queries.toml.

# [connections.local_sqlite]
# db_type = "sqlite"
# conn = "./sqvue_demo.db"

# [connections.local_mysql]
# db_type = "mysql"
# conn = "user:password@tcp(localhost:3306)/my_database?parseTime=true"
`

// DefaultPath returns the platform's standard per-user configuration location.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "sqvue", "config.toml"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config directory: %w", err)
	}
	return filepath.Join(dir, "sqvue", "config.toml"), nil
}

// QueryStorePath returns the path used for SQL history and saved queries.
func QueryStorePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "queries.toml")
}

// LoadQueryStore loads SQL workflow data from path. A missing store is empty
// and is written only after the user saves a query or runs SQL.
func LoadQueryStore(path string) (*QueryStore, error) {
	store := &QueryStore{Profiles: make(map[string]QueryProfile), path: path}
	if _, err := toml.DecodeFile(path, store); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return store, nil
		}
		return nil, fmt.Errorf("load query store: %w", err)
	}
	if store.Profiles == nil {
		store.Profiles = make(map[string]QueryProfile)
	}
	store.path = path
	return store, nil
}

// History returns a copy of a profile's SQL history, oldest first.
func (s *QueryStore) History(profile string) []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.Profiles[profile].History...)
}

// QueryNames returns a profile's saved query names in stable order.
func (s *QueryStore) QueryNames(profile string) []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.Profiles[profile].Queries))
	for name := range s.Profiles[profile].Queries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (s *QueryStore) Query(profile, name string) (string, bool) {
	if s == nil {
		return "", false
	}
	query, ok := s.Profiles[profile].Queries[name]
	return query, ok
}

// AddHistory records SQL once at the end of the profile's history.
func (s *QueryStore) AddHistory(profile, sql string) error {
	if strings.TrimSpace(sql) == "" {
		return nil
	}
	p := s.profile(profile)
	if n := len(p.History); n > 0 && p.History[n-1] == sql {
		return nil
	}
	p.History = append(p.History, sql)
	if len(p.History) > queryHistoryLimit {
		p.History = append([]string(nil), p.History[len(p.History)-queryHistoryLimit:]...)
	}
	return s.replaceProfile(profile, p)
}

func (s *QueryStore) SaveQuery(profile, name, sql string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("saved query name is required")
	}
	if strings.TrimSpace(sql) == "" {
		return errors.New("saved query SQL is required")
	}
	p := s.profile(profile)
	p.Queries[name] = sql
	return s.replaceProfile(profile, p)
}

func (s *QueryStore) RenameQuery(profile, oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("saved query name is required")
	}
	p := s.profile(profile)
	sql, ok := p.Queries[oldName]
	if !ok {
		return fmt.Errorf("saved query %q was not found", oldName)
	}
	if oldName != newName {
		if _, exists := p.Queries[newName]; exists {
			return fmt.Errorf("saved query %q already exists", newName)
		}
		delete(p.Queries, oldName)
		p.Queries[newName] = sql
	}
	return s.replaceProfile(profile, p)
}

func (s *QueryStore) DeleteQuery(profile, name string) error {
	p := s.profile(profile)
	if _, ok := p.Queries[name]; !ok {
		return fmt.Errorf("saved query %q was not found", name)
	}
	delete(p.Queries, name)
	return s.replaceProfile(profile, p)
}

func (s *QueryStore) profile(name string) QueryProfile {
	p := s.Profiles[name]
	p.History = append([]string(nil), p.History...)
	queries := make(map[string]string, len(p.Queries))
	for key, value := range p.Queries {
		queries[key] = value
	}
	p.Queries = queries
	return p
}

func (s *QueryStore) replaceProfile(name string, profile QueryProfile) error {
	previous, existed := s.Profiles[name]
	s.Profiles[name] = profile
	if err := s.save(); err != nil {
		if existed {
			s.Profiles[name] = previous
		} else {
			delete(s.Profiles, name)
		}
		return err
	}
	return nil
}

func (s *QueryStore) save() error {
	if s == nil || s.path == "" {
		return errors.New("query store path is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create query store directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".queries-*.toml")
	if err != nil {
		return fmt.Errorf("create query store: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set query store permissions: %w", err)
	}
	if err := toml.NewEncoder(tmp).Encode(s); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode query store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close query store: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("save query store: %w", err)
	}
	return nil
}

// LoadOrCreate loads path, creating a commented configuration template when it
// does not exist. The returned bool reports whether the template was created.
func LoadOrCreate(path string) (File, bool, error) {
	file, err := load(path)
	if err == nil {
		return file, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return File{}, false, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return File{}, false, fmt.Errorf("create config directory: %w", err)
	}
	created, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		if _, writeErr := created.WriteString(defaultFile); writeErr != nil {
			_ = created.Close()
			return File{}, false, fmt.Errorf("write default config: %w", writeErr)
		}
		if closeErr := created.Close(); closeErr != nil {
			return File{}, false, fmt.Errorf("close default config: %w", closeErr)
		}
		return File{}, true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return File{}, false, fmt.Errorf("create default config: %w", err)
	}

	file, err = load(path)
	if err != nil {
		return File{}, false, err
	}
	return file, false, nil
}

func load(path string) (File, error) {
	var file File
	metadata, err := toml.DecodeFile(path, &file)
	if err != nil {
		return File{}, err
	}
	if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return File{}, fmt.Errorf("unknown configuration key(s): %s", strings.Join(keys, ", "))
	}
	if _, err := file.Settings.ResolveTheme(); err != nil {
		return File{}, err
	}
	return file, nil
}

// ResolveTheme returns the selected built-in theme. An unset setting retains
// the default appearance.
func (s Settings) ResolveTheme() (theme.Theme, error) {
	styles, err := theme.ByName(s.Theme)
	if err != nil {
		return theme.Theme{}, fmt.Errorf("settings.theme: %w", err)
	}
	return styles, nil
}

// Profile returns a named connection profile.
func (f File) Profile(name string) (DBProfile, error) {
	profile, ok := f.Connections[name]
	if !ok {
		return DBProfile{}, fmt.Errorf("connection profile %q was not found", name)
	}
	return profile, nil
}

// DefaultDBProfile contains the existing command-line defaults.
func DefaultDBProfile() DBProfile {
	return DBProfile{
		DBType:  "postgres",
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		SSLMode: "require",
		Timeout: 5 * time.Second,
	}
}

// Merge applies non-zero connection fields from overrides to p.
func (p DBProfile) Merge(overrides DBProfile) DBProfile {
	if overrides.DBType != "" {
		p.DBType = overrides.DBType
	}
	if overrides.ConnString != "" {
		p.ConnString = overrides.ConnString
	}
	if overrides.Host != "" {
		p.Host = overrides.Host
	}
	if overrides.Port != 0 {
		p.Port = overrides.Port
	}
	if overrides.User != "" {
		p.User = overrides.User
	}
	if overrides.Password != "" {
		p.Password = overrides.Password
	}
	if overrides.Database != "" {
		p.Database = overrides.Database
	}
	if overrides.SSLMode != "" {
		p.SSLMode = overrides.SSLMode
	}
	if overrides.RetainQueryHistory {
		p.RetainQueryHistory = true
	}
	if overrides.Timeout != 0 {
		p.Timeout = overrides.Timeout
	}
	return p
}

// ValidateFor checks the connection requirements for a selected driver.
func (p DBProfile) ValidateFor(dbType string) error {
	if dbType == "sqlite" {
		if p.ConnString == "" {
			return errors.New("provide --conn or a profile conn for SQLite")
		}
		if p.Timeout <= 0 {
			return errors.New("timeout must be > 0")
		}
		return nil
	}
	if dbType == "mysql" {
		if p.ConnString == "" {
			return errors.New("provide --conn or a profile conn for MySQL")
		}
		if p.Timeout <= 0 {
			return errors.New("timeout must be > 0")
		}
		return nil
	}
	return p.Validate()
}

func (p DBProfile) Validate() error {
	if p.ConnString == "" {
		if p.Host == "" || p.User == "" || p.Database == "" {
			return errors.New("provide --conn (or DATABASE_URL), or --host, --user, and --db")
		}
		if p.Port <= 0 || p.Port > 65535 {
			return errors.New("port must be between 1 and 65535")
		}
	}
	if p.Timeout <= 0 {
		return errors.New("timeout must be > 0")
	}
	return nil
}
