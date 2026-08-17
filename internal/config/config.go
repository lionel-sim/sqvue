package config

import (
	"errors"
	"time"
)

type DBProfile struct {
	ConnString string
	Timeout    time.Duration
}

func (p DBProfile) Validate() error {
	if p.ConnString == "" {
		return errors.New("no connection string provided; set --conn or DATABASE_URL")
	}
	if p.Timeout <= 0 {
		return errors.New("timeout must be > 0")
	}
	return nil
}
