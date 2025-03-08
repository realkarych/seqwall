package main

import (
	"fmt"
	"os"

	"github.com/realkarych/seqwall/pkg/seqwall"
	"github.com/spf13/cobra"
)

var (
	migrationsPath string
	testSchema     bool
	depth          int
	migrateUp      string
	migrateDown    string
	postgresUrl    string
)

var rootCmd = &cobra.Command{
	Use:   "seqwall",
	Short: "Seqwall — CLI for testing your RawSQL migrations",
	Long:  "Seqwall — CLI for testing your RawSQL migrations. Check https://github.com/realkarych/seqwall",
}

var staircaseCmd = &cobra.Command{
	Use:   "staircase",
	Short: "Launch staircase testing",
	Long:  "Launch staircase testing to check the schema consistency",
	PreRun: func(cmd *cobra.Command, args []string) {
		if postgresUrl == "" {
			postgresUrl = os.Getenv("POSTGRES_URL")
		}
		if postgresUrl == "" {
			fmt.Fprintln(os.Stderr, "Error: postgres-url flag not provided and POSTGRES_URL environment variable is not set")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cli := seqwall.NewStaircaseCli(migrationsPath, testSchema, depth, migrateUp, migrateDown, postgresUrl)
		cli.Run()
	},
}

func init() {
	staircaseCmd.Flags().StringVarP(&migrationsPath, "migrations", "m", "", "Path for migrations")
	staircaseCmd.Flags().BoolVar(&testSchema, "test-schema", true, "Check schema consistency or not (default: true)")
	staircaseCmd.Flags().IntVarP(&depth, "depth", "d", 0, "Depth of staircase testing (0 - all migrations)")
	staircaseCmd.Flags().StringVar(&migrateUp, "migrate-up", "", "Migrate up command")
	staircaseCmd.Flags().StringVar(&migrateDown, "migrate-down", "", "Migrate down command")
	staircaseCmd.Flags().StringVar(&postgresUrl, "postgres-url", "", "Postgres URL (default: POSTGRES_URL environment variable)")

	staircaseCmd.MarkFlagRequired("migrations")
	staircaseCmd.MarkFlagRequired("migrate-up")
	staircaseCmd.MarkFlagRequired("migrate-down")

	rootCmd.AddCommand(staircaseCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
