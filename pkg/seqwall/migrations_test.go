package seqwall

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func createTempFiles(t *testing.T, dir string, names []string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if strings.HasSuffix(name, "/") {
			if err := os.Mkdir(filepath.Join(dir, strings.TrimSuffix(name, "/")), 0o755); err != nil {
				t.Fatalf("failed to create subdir %s: %v", name, err)
			}
		} else {
			if err := os.WriteFile(path, []byte("-- migration"), 0o644); err != nil {
				t.Fatalf("failed to write file %s: %v", name, err)
			}
		}
	}
}

func TestLoadMigrations_Success(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, []string{"b.sql", "a.sql", "c.txt", "subdir/"})
	sub := filepath.Join(dir, "subdir")
	if err := os.WriteFile(filepath.Join(sub, "x.sql"), []byte("-- sub migration"), 0o644); err != nil {
		t.Fatalf("failed to write nested migration: %v", err)
	}

	migrations, err := loadMigrations(dir, ".sql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{
		filepath.Join(dir, "a.sql"),
		filepath.Join(dir, "b.sql"),
	}
	if !reflect.DeepEqual(migrations, expected) {
		t.Errorf("migrations = %v; want %v", migrations, expected)
	}
}

func TestLoadMigrations_NoMatch(t *testing.T) {
	dir := t.TempDir()
	createTempFiles(t, dir, []string{"foo.txt", "bar.md"})
	migrations, err := loadMigrations(dir, ".sql")
	if err == nil {
		t.Fatal("expected error for no matching migration files, got nil")
	}
	if migrations != nil {
		t.Errorf("expected nil migrations slice on error, got %v", migrations)
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "ext=.sql") || !strings.Contains(errMsg, dir) {
		t.Errorf("error message = %q; want it to mention ext and dir", errMsg)
	}
}

func TestLoadMigrations_DirNotExist(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "does_not_exist")
	_, err := loadMigrations(nonexistent, ".sql")
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
	if !errorsIs(err, fs.ErrNotExist) {
		t.Errorf("error = %v; want it to wrap ErrNotExist", err)
	}
}

func errorsIs(err, target error) bool {
	return errors.Is(err, target)
}
