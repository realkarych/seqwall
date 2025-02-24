package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/realkarych/seqwall/pkg/seqwall"
)

func main() {
	config, err := parseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Starts with config: %+v\n", config)
}

func parseConfig() (seqwall.SeqwallConfig, error) {
	var configPath, environment, migrationName string

	flag.StringVar(&configPath, "config", "seqwall.yaml", "Path to seqwall.yaml")
	flag.StringVar(&configPath, "c", "seqwall.yaml", "Path to seqwall.yaml")
	flag.StringVar(&environment, "env", "default", "Environment in seqwall.yaml to use")
	flag.StringVar(&environment, "e", "default", "Environment in seqwall.yaml to use")
	flag.StringVar(&migrationName, "migration", "", "Last migration name")
	flag.StringVar(&migrationName, "m", "", "Last migration name")

	flag.Parse()

	file, err := os.Open(configPath)
	if err != nil {
		return seqwall.SeqwallConfig{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	configs, err := seqwall.NewYamlParser().Parse(file)
	if err != nil {
		return seqwall.SeqwallConfig{}, err
	}

	conf, exists := configs[environment]
	if !exists {
		return seqwall.SeqwallConfig{}, fmt.Errorf("environment %q not found in config", environment)
	}

	if migrationName != "" {
		conf.MigrationName = migrationName
	} else {
		lastMigrationName, err := getLastMigrationName(conf.MigrationsDir)
		if err != nil {
			return seqwall.SeqwallConfig{}, err
		}
		conf.MigrationName = lastMigrationName
	}

	return conf, nil
}

func getLastMigrationName(migrationsDir string) (string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	var migrationFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}

	if len(migrationFiles) == 0 {
		return "", fmt.Errorf("no .sql migration files found in directory %s", migrationsDir)
	}

	sort.Strings(migrationFiles)
	return migrationFiles[len(migrationFiles)-1], nil
}
