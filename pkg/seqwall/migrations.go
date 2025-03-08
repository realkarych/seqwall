package seqwall

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func getLastMigration(migrationsDir string) (string, error) {
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
