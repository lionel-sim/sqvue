package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type DBProfile struct {
	ConnString string        `toml:"conn"`
	Host       string        `toml:"host"`
	Port       int           `toml:"port"`
	User       string        `toml:"user"`
	Password   string        `toml:"password"`
	Database   string        `toml:"database"`
	SSLMode    string        `toml:"sslmode"`
	Timeout    time.Duration `toml:"-"`
}

// File is sqvue's on-disk configuration. Connections are keyed by profile name.
type File struct {
	Settings    Settings             `toml:"settings"`
	Connections map[string]DBProfile `toml:"connections"`
}

type Settings struct {
	DefaultProfile string `toml:"default_profile"`
}

const defaultFile = `# sqvue configuration
#
# Define named PostgreSQL connections here, then select one with
# settings.default_profile or --profile. Do not commit passwords to source control.

[settings]
# default_profile = "local"

# [connections.local]
# host = "localhost"
# port = 5432
# user = "postgres"
# database = "my_database"
# sslmode = "disable"
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
	return file, nil
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
		Host:    "localhost",
		Port:    5432,
		User:    "postgres",
		SSLMode: "require",
		Timeout: 5 * time.Second,
	}
}

// Merge applies non-zero connection fields from overrides to p.
func (p DBProfile) Merge(overrides DBProfile) DBProfile {
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
	if overrides.Timeout != 0 {
		p.Timeout = overrides.Timeout
	}
	return p
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
