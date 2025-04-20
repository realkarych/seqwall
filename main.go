package main

import (
	"log"
	"os"

	"github.com/realkarych/seqwall/pkg/seqwall"
	"github.com/spf13/cobra"
)

const (
	ExitOk           = 0
	ExitRuntimeError = 1
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
	Short: "Seqwall — CLI for testing your PostgreSQL migrations",
	Long:  "Seqwall — CLI for testing your PostgreSQL migrations. Check https://github.com/realkarych/seqwall",
}

var staircaseCmd = &cobra.Command{
	Use:   "staircase",
	Short: "Launch staircase testing",
	Long:  "Launch staircase testing to check the schema consistency. Remember that migrations should be in lexicographical order",
	PreRun: func(cmd *cobra.Command, args []string) {
		if postgresUrl == "" {
			postgresUrl = os.Getenv("DATABASE_URL")
		}
		if postgresUrl == "" {
			log.Fatalf("Error: postgres-url flag not provided and DATABASE_URL environment variable is not set")
			os.Exit(ExitRuntimeError)
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
	staircaseCmd.Flags().StringVar(&postgresUrl, "postgres-url", "", "Postgres URL (default: DATABASE_URL environment variable)")

	staircaseCmd.MarkFlagRequired("migrations")
	staircaseCmd.MarkFlagRequired("migrate-up")
	staircaseCmd.MarkFlagRequired("migrate-down")

	rootCmd.AddCommand(staircaseCmd)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("From panic: %v", r)
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}
	log.Println("Execution completed successfully.")
	os.Exit(ExitOk)
}
