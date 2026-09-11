package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"

	"github.com/realkarych/seqwall/pkg/seqwall"
	"github.com/spf13/cobra"
)

const (
	exitOK    = 0
	exitError = 1
)

var Version = "dev"

func resolveVersion(linkerVersion string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if linkerVersion != "dev" {
		return linkerVersion
	}
	buildInfo, ok := readBuildInfo()
	if ok && buildInfo != nil {
		for _, setting := range buildInfo.Settings {
			if setting.Key == "vcs.revision" {
				return "dev"
			}
		}
		if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			return buildInfo.Main.Version
		}
	}
	return "dev"
}

type StaircaseOptions struct {
	MigrationsPath         string   `json:"migrations-path"`
	UpgradeCmd             string   `json:"upgrade"`
	DowngradeCmd           string   `json:"downgrade"`
	PostgresURL            string   `json:"postgres-url"`
	MigrationsExtension    string   `json:"migrations-extension"`
	Schemas                []string `json:"schemas"`
	Depth                  int      `json:"depth"`
	CompareSchemaSnapshots bool     `json:"compare-snapshots"`
}

func main() {
	opts := &StaircaseOptions{}
	exitCode := exitOK
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic: %v", r)
			exitCode = exitError
		}
		os.Exit(exitCode)
	}()
	if err := newRootCmd(opts).Execute(); err != nil {
		log.Println(err)
		exitCode = exitError
	}
}

func newRootCmd(opts *StaircaseOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           "seqwall",
		Short:         "Seqwall — CLI for testing PostgreSQL migrations",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       resolveVersion(Version, debug.ReadBuildInfo),
	}
	root.SetVersionTemplate("seqwall {{.Version}}\n")
	root.AddCommand(newStaircaseCmd(opts))
	return root
}

func newStaircaseCmd(opts *StaircaseOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "staircase",
		Short:   "Launch staircase testing",
		Long:    "Launch staircase testing",
		Args:    cobra.NoArgs,
		PreRunE: invalidateOptions(opts),
		RunE:    staircaseRun(opts),
	}
	bindStaircaseFlags(cmd, opts)
	markRequired(cmd, "migrations-path", "upgrade", "downgrade")
	cmd.Flags().SortFlags = false

	return cmd
}

func invalidateOptions(opts *StaircaseOptions) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		if strings.TrimSpace(opts.MigrationsPath) == "" {
			return fmt.Errorf("--migrations-path must not be empty")
		}
		if strings.TrimSpace(opts.UpgradeCmd) == "" {
			return fmt.Errorf("--upgrade must not be empty")
		}
		if strings.TrimSpace(opts.DowngradeCmd) == "" {
			return fmt.Errorf("--downgrade must not be empty")
		}
		if opts.Depth < 0 {
			return fmt.Errorf("--depth must be zero or greater")
		}
		if opts.PostgresURL == "" {
			opts.PostgresURL = os.Getenv("DATABASE_URL")
		}
		if opts.PostgresURL == "" {
			return seqwall.ErrPostgresURLRequired()
		}
		return nil
	}
}

func staircaseRun(opts *StaircaseOptions) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, _ []string) error {
		worker := seqwall.NewStaircaseWorker(
			opts.MigrationsPath,
			opts.CompareSchemaSnapshots,
			opts.Depth,
			opts.UpgradeCmd,
			opts.DowngradeCmd,
			opts.PostgresURL,
			opts.Schemas,
			opts.MigrationsExtension,
		)
		return worker.Run()
	}
}

func bindStaircaseFlags(cmd *cobra.Command, opts *StaircaseOptions) {
	cmd.Flags().StringVar(&opts.PostgresURL, "postgres-url", "", "PostgreSQL connection URL (defaults to DATABASE_URL)")
	cmd.Flags().StringVar(&opts.MigrationsPath, "migrations-path", "", "Directory containing lexicographically ordered migration files")
	cmd.Flags().StringVar(&opts.UpgradeCmd, "upgrade", "", "Command that applies exactly one migration")
	cmd.Flags().StringVar(&opts.DowngradeCmd, "downgrade", "", "Command that reverts exactly one migration")
	cmd.Flags().BoolVar(&opts.CompareSchemaSnapshots, "test-snapshots", true, "Compare schema snapshots")
	cmd.Flags().StringArrayVar(&opts.Schemas, "schema", []string{"public"}, "Schema to include in testing (repeatable)")
	cmd.Flags().IntVar(&opts.Depth, "depth", 0, "Number of migrations to test (0 means all)")
	cmd.Flags().StringVar(&opts.MigrationsExtension, "migrations-extension", ".sql", "Migration filename extension")
}

func markRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(fmt.Sprintf("internal: flag %q should be declared above: %v", name, err))
		}
	}
}
