package seqwall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadMigrations(migrationsPath, extension string) ([]string, error) {
	var migrationFiles []string
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return migrationFiles, fmt.Errorf("failed to read migrations directory %s: %w", migrationsPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), extension) {
			fullPath := filepath.Join(migrationsPath, entry.Name())
			migrationFiles = append(migrationFiles, fullPath)
		}
	}

	if len(migrationFiles) == 0 {
		return migrationFiles, fmt.Errorf("no %s migration files found in directory %s", extension, migrationsPath)
	}

	sort.Strings(migrationFiles)
	return migrationFiles, nil
}
