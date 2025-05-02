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
	migrationsPath         string
	compareSchemaSnapshots bool
	depth                  int
	upgradeCmd             string
	downgradeCmd           string
	postgresURL            string
	schemas                []string
	migrationsExtension    string
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
		Long:  "Launch staircase testing",
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
				compareSchemaSnapshots,
				depth,
				upgradeCmd,
				downgradeCmd,
				postgresURL,
				schemas,
				migrationsExtension,
			)
			return worker.Run()
		},
	}

	cmd.Flags().StringVar(
		&postgresURL,
		"postgres-url",
		"",
		"PostgreSQL URL (fallback: $DATABASE_URL)",
	)
	cmd.Flags().StringVar(
		&migrationsPath,
		"migrations-path",
		"",
		"Path to migrations (required). Migrations must be in lexicographical order",
	)
	cmd.Flags().StringVar(
		&upgradeCmd,
		"upgrade",
		"",
		"Shell command that applies next migration (required)",
	)
	cmd.Flags().StringVar(
		&downgradeCmd,
		"downgrade",
		"",
		"Shell command that reverts current migration (required)",
	)
	cmd.Flags().BoolVar(
		&compareSchemaSnapshots,
		"test-snapshots",
		true,
		"Compare schema snapshots (default true). If false, only checks fact that migrations are applied / reverted with no errors",
	)
	cmd.Flags().StringArrayVar(
		&schemas,
		"schema",
		[]string{"public"},
		"Schemas to test (default: public)",
	)
	cmd.Flags().IntVar(
		&depth,
		"depth",
		0,
		"Depth of staircase testing (0 = all)",
	)
	cmd.Flags().StringVar(
		&migrationsExtension,
		"migrations-extension",
		".sql",
		"Extension of migration files (default: .sql)",
	)

	_ = cmd.MarkFlagRequired("migrations-path")
	_ = cmd.MarkFlagRequired("upgrade")
	_ = cmd.MarkFlagRequired("downgrade")

	return cmd
}
