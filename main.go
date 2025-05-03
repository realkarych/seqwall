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
		Use:     "staircase",
		Short:   "Launch staircase testing",
		Long:    "Launch staircase testing",
		PreRunE: invalidateOptions,
		RunE:    staircaseRun,
	}

	bindStaircaseFlags(cmd)
	markRequired(cmd, "migrations-path", "upgrade", "downgrade")
	cmd.Flags().SortFlags = false

	return cmd
}

func invalidateOptions(cmd *cobra.Command, _ []string) error {
	if postgresURL == "" {
		postgresURL = os.Getenv("DATABASE_URL")
	}
	if postgresURL == "" {
		return seqwall.ErrPostgresURLRequired()
	}
	return nil
}

func staircaseRun(cmd *cobra.Command, _ []string) error {
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
}

func bindStaircaseFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&postgresURL,
		"postgres-url",
		"",
		"PostgreSQL URL (required OR fallback — $DATABASE_URL env variable)",
	)
	cmd.Flags().StringVar(
		&migrationsPath,
		"migrations-path",
		"",
		"Path to migrations. Migrations must be in lexicographical order (required)",
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
		"Compare schema snapshots. If false, only checks fact that migrations are applied / reverted with no errors",
	)
	cmd.Flags().StringArrayVar(
		&schemas,
		"schema",
		[]string{"public"},
		"Schemas to test",
	)
	cmd.Flags().IntVar(
		&depth,
		"depth",
		0,
		"Depth of staircase testing (0 = all). If depth is N, only the last N migrations will be processed",
	)
	cmd.Flags().StringVar(
		&migrationsExtension,
		"migrations-extension",
		".sql",
		"Extension of migration files",
	)
}

func markRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(fmt.Sprintf("internal: flag %q should be declared above: %v", name, err))
		}
	}
}
