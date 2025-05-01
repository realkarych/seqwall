package main

import (
	"fmt"
	"log"
	"os"

	"github.com/realkarych/seqwall/pkg/seqwall"
	"github.com/spf13/cobra"
)

const (
	exitOK    = 0
	exitError = 1
)

var (
	migrationsPath string
	testSchema     bool
	depth          int
	migrateUp      string
	migrateDown    string
	postgresURL    string
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic: %v", r)
			os.Exit(exitError)
		}
	}()

	if err := newRootCmd().Execute(); err != nil {
		log.Println(err)
		os.Exit(exitError)
	}
	os.Exit(exitOK)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "seqwall",
		Short:         "Seqwall — CLI for testing PostgreSQL migrations",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newStaircaseCmd())
	return root
}

func newStaircaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "staircase",
		Short: "Launch staircase testing",
		Long:  "Launch staircase testing to check schema consistency. Migrations must be in lexicographical order.",
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if postgresURL == "" {
				postgresURL = os.Getenv("DATABASE_URL")
			}
			if postgresURL == "" {
				return fmt.Errorf("--postgres-url flag or DATABASE_URL env is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			worker := seqwall.NewStaircaseWorker(
				migrationsPath,
				testSchema,
				depth,
				migrateUp,
				migrateDown,
				postgresURL,
			)
			return worker.Run()
		},
	}

	cmd.Flags().StringVarP(&migrationsPath, "migrations", "m", "", "Path to migrations (required)")
	cmd.Flags().BoolVar(&testSchema, "test-schema", true, "Compare schema snapshots (default true)")
	cmd.Flags().IntVarP(&depth, "depth", "d", 0, "Depth of staircase testing (0 = all)")
	cmd.Flags().StringVar(&migrateUp, "migrate-up", "", "Shell command that applies a migration (required)")
	cmd.Flags().StringVar(&migrateDown, "migrate-down", "", "Shell command that reverts a migration (required)")
	cmd.Flags().StringVar(&postgresURL, "postgres-url", "", "PostgreSQL URL (fallback: $DATABASE_URL)")

	_ = cmd.MarkFlagRequired("migrations")
	_ = cmd.MarkFlagRequired("migrate-up")
	_ = cmd.MarkFlagRequired("migrate-down")

	return cmd
}
