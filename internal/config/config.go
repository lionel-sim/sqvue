package config

import (
	"errors"
	"time"
)

type DBProfile struct {
	ConnString string
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	Timeout    time.Duration
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
