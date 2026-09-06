package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"sqvue/internal/config"
	"sqvue/internal/db"
	_ "sqvue/internal/db/mysql"
	_ "sqvue/internal/db/postgres"
	_ "sqvue/internal/db/sqlite"
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
	dbTypeFlag   string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&dbTypeFlag, "db-type", "postgres", "Database type (postgres, sqlite, or mysql)")
	rootCmd.PersistentFlags().StringVar(&connFlag, "conn", "", "Database connection string or SQLite database path")
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
	file, created, path, err := loadConfig()
	if err != nil {
		return err
	}
	cfg, err := profileFromFile(file, cmd)
	if err != nil {
		return err
	}
	styles, err := file.Settings.ResolveTheme()
	if err != nil {
		return err
	}
	if created {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Created config file at %s\n", path); err != nil {
			return fmt.Errorf("report config creation: %w", err)
		}
	}
	queryStore, err := config.LoadQueryStore(config.QueryStorePath(path))
	if err != nil {
		return err
	}
	dbType := db.DbType(cfg.DBType)
	if err := cfg.ValidateFor(string(dbType)); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer cancel()

	client, err := db.NewClientWithConfig(ctx, db.ConnectConfig{
		DbType:   dbType,
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
	m := tui.New(tui.Options{
		Client:          client,
		Timeout:         cfg.Timeout,
		ExportDirectory: exportDirectory(file.Settings.ExportDirectory),
		Theme:           styles,
		QueryStore:      queryStore,
		ProfileName:     selectedProfileName(file),
		Profiles:        tuiProfiles(file),
	})

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if typed, ok := finalModel.(tui.Model); ok {
		_ = typed.Close()
	} else {
		_ = client.Close()
	}
	if err != nil {
		log.Printf("tui error: %v", err)
		return err
	}
	return nil
}

func tuiProfiles(file config.File) []tui.ConnectionProfile {
	names := make([]string, 0, len(file.Connections))
	for name := range file.Connections {
		names = append(names, name)
	}
	sort.Strings(names)
	profiles := make([]tui.ConnectionProfile, 0, len(names))
	for _, name := range names {
		profile := config.DefaultDBProfile().Merge(file.Connections[name])
		profiles = append(profiles, tui.ConnectionProfile{
			Name:    name,
			Config:  db.ConnectConfig{DbType: db.DbType(profile.DBType), DSN: profile.ConnString, Host: profile.Host, Port: profile.Port, User: profile.User, Password: profile.Password, Database: profile.Database, SSLMode: profile.SSLMode},
			Timeout: profile.Timeout,
		})
	}
	return profiles
}

func selectedProfileName(file config.File) string {
	if profileFlag != "" {
		return profileFlag
	}
	if file.Settings.DefaultProfile != "" {
		return file.Settings.DefaultProfile
	}
	return "default"
}

func loadProfile(cmd *cobra.Command) (config.DBProfile, bool, string, error) {
	file, created, path, err := loadConfig()
	if err != nil {
		return config.DBProfile{}, false, "", err
	}
	cfg, err := profileFromFile(file, cmd)
	return cfg, created, path, err
}

func loadConfig() (config.File, bool, string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.File{}, false, "", err
	}
	file, created, err := config.LoadOrCreate(path)
	if err != nil {
		return config.File{}, false, "", fmt.Errorf("load config: %w", err)
	}
	return file, created, path, nil
}

func profileFromFile(file config.File, cmd *cobra.Command) (config.DBProfile, error) {
	cfg := config.DefaultDBProfile()
	profileName := firstNonEmpty(profileFlag, file.Settings.DefaultProfile)
	if profileName != "" {
		profile, err := file.Profile(profileName)
		if err != nil {
			return config.DBProfile{}, err
		}
		cfg = cfg.Merge(profile)
	}
	if cmd.Flags().Changed("db-type") {
		cfg.DBType = dbTypeFlag
	}

	if connFlag != "" {
		cfg.ConnString = connFlag
	} else if profileFlag == "" && cfg.DBType == string(db.DbTypePostgres) {
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
	return cfg, nil
}

func exportDirectory(configured string) string {
	if configured == "" {
		configured = "."
	}
	path, err := filepath.Abs(configured)
	if err != nil {
		return configured
	}
	return path
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
