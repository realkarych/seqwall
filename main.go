package main

import (
	"fmt"
	"os"

	"github.com/realkarych/seqwall/pkg/seqwall"
	"github.com/spf13/cobra"
)

var (
	migrations string
	testSchema bool
	depth      int
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
	Run: func(cmd *cobra.Command, args []string) {
		StaircaseCli := seqwall.NewStaircaseCli(migrations, testSchema, depth)
		StaircaseCli.Run()
	},
}

func init() {
	staircaseCmd.Flags().StringVarP(&migrations, "migrations", "m", "", "Path for migrations")
	staircaseCmd.Flags().BoolVar(&testSchema, "test-schema", true, "Check schema consistency or not (default: true)")
	staircaseCmd.Flags().IntVarP(&depth, "depth", "d", 0, "Depth of staircase testing (0 - all migrations)")

	staircaseCmd.MarkFlagRequired("migrations")

	rootCmd.AddCommand(staircaseCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
