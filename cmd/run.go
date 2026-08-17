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
	connFlag    string
	pageSize    int
	timeoutFlag time.Duration
)

func init() {
	rootCmd.PersistentFlags().StringVar(&connFlag, "conn", "", "Postgres connection string (or set DATABASE_URL)")
	rootCmd.PersistentFlags().IntVar(&pageSize, "page-size", 50, "rows per page")
	rootCmd.PersistentFlags().DurationVar(&timeoutFlag, "timeout", 5*time.Second, "query timeout")

	rootCmd.RunE = run
}

func run(cmd *cobra.Command, args []string) error {
	cfg := config.DBProfile{
		ConnString: firstNonEmpty(connFlag, os.Getenv("DATABASE_URL")),
		PageSize:   pageSize,
		Timeout:    timeoutFlag,
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	client, err := db.NewClient(ctx, cfg.ConnString)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	m := tui.New(tui.Options{
		Client:   client,
		PageSize: cfg.PageSize,
		Timeout:  cfg.Timeout,
	})

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Printf("tui error: %v", err)
		return err
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
