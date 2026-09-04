package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"sqvue/internal/config"
	"sqvue/internal/db"
	_ "sqvue/internal/db/postgres"
	"sqvue/internal/tui"
)

var (
	connFlag     string
	timeoutFlag  time.Duration
	hostFlag     string
	portFlag     int
	userFlag     string
	passwordFlag string
	databaseFlag string
	sslModeFlag  string
	profileFlag  string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&connFlag, "conn", "", "Postgres connection string (or set DATABASE_URL)")
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "Named connection profile from the config file")
	rootCmd.PersistentFlags().StringVar(&hostFlag, "host", "localhost", "Postgres host")
	rootCmd.PersistentFlags().IntVar(&portFlag, "port", 5432, "Postgres port")
	rootCmd.PersistentFlags().StringVar(&userFlag, "user", "postgres", "Postgres user")
	rootCmd.PersistentFlags().StringVar(&passwordFlag, "password", "", "Postgres password")
	rootCmd.PersistentFlags().StringVar(&databaseFlag, "db", "", "Postgres database")
	rootCmd.PersistentFlags().StringVar(&sslModeFlag, "sslmode", "require", "Postgres TLS mode (disable, require, verify-ca, verify-full)")
	rootCmd.PersistentFlags().DurationVar(&timeoutFlag, "timeout", 5*time.Second, "connection and query timeout")

	rootCmd.RunE = run
}

func run(cmd *cobra.Command, args []string) error {
	cfg, created, path, err := loadProfile(cmd)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(cmd.ErrOrStderr(), "Created config file at %s\n", path)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer cancel()

	client, err := db.NewClientWithConfig(ctx, db.ConnectConfig{
		DbType:   db.DbTypePostgres,
		DSN:      cfg.ConnString,
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		Database: cfg.Database,
		SSLMode:  cfg.SSLMode,
	})
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	m := tui.New(tui.Options{
		Client:  client,
		Timeout: cfg.Timeout,
	})

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("tui error: %v", err)
		return err
	}
	return nil
}

func loadProfile(cmd *cobra.Command) (config.DBProfile, bool, string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.DBProfile{}, false, "", err
	}
	file, created, err := config.LoadOrCreate(path)
	if err != nil {
		return config.DBProfile{}, false, "", fmt.Errorf("load config: %w", err)
	}

	cfg := config.DefaultDBProfile()
	profileName := firstNonEmpty(profileFlag, file.Settings.DefaultProfile)
	if profileName != "" {
		profile, err := file.Profile(profileName)
		if err != nil {
			return config.DBProfile{}, false, "", err
		}
		cfg = cfg.Merge(profile)
	}

	if connFlag != "" {
		cfg.ConnString = connFlag
	} else if profileFlag == "" {
		if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
			cfg.ConnString = dsn
		}
	}
	flags := cmd.Flags()
	if flags.Changed("host") {
		cfg.Host = hostFlag
	}
	if flags.Changed("port") {
		cfg.Port = portFlag
	}
	if flags.Changed("user") {
		cfg.User = userFlag
	}
	if flags.Changed("password") {
		cfg.Password = passwordFlag
	}
	if flags.Changed("db") {
		cfg.Database = databaseFlag
	}
	if flags.Changed("sslmode") {
		cfg.SSLMode = sslModeFlag
	}
	if flags.Changed("timeout") {
		cfg.Timeout = timeoutFlag
	}
	return cfg, created, path, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
