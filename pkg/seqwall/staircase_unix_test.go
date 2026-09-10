//go:build !windows
// +build !windows

package seqwall

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/realkarych/seqwall/pkg/driver"
)

func TestCalculateStairDepth(t *testing.T) {
	t.Parallel()

	migs := []string{"1.sql", "2.sql", "3.sql", "4.sql", "5.sql"}
	cases := []struct {
		name  string
		depth int
		want  int
	}{
		{"depth 0 -> all", 0, 5},
		{"depth less than len", 3, 3},
		{"depth bigger than len", 10, 5},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := &StaircaseWorker{depth: c.depth}
			got := w.calculateStairDepth(migs)
			if got != c.want {
				t.Fatalf("calculateStairDepth() got %d, want %d", got, c.want)
			}
		})
	}
}

func TestBuildSchemaCond(t *testing.T) {
	t.Parallel()

	cases := []struct {
		schemas []string
		col     string
		want    string
	}{
		{nil, "table_schema", "table_schema = 'public'"},
		{[]string{"public"}, "table_schema", "table_schema = 'public'"},
		{[]string{"public", "extra"}, "tc.table_schema", "tc.table_schema IN ('public', 'extra')"},
	}

	for _, c := range cases {
		c := c
		name := strings.Join(c.schemas, "+")
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := &StaircaseWorker{schemas: c.schemas}
			got := w.buildSchemaCond(c.col)
			if got != c.want {
				t.Fatalf("buildSchemaCond() got %q, want %q", got, c.want)
			}
		})
	}
}

func TestExecuteCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "ok.sh")
	body := "#!/bin/sh\necho OK"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	w := &StaircaseWorker{}

	out, err := w.executeCommand(scriptPath, "dummy")
	if err != nil {
		t.Fatalf("executeCommand() unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Fatalf("executeCommand() output = %q, want 'OK'", out)
	}
	failPath := filepath.Join(dir, "fail.sh")
	bodyFail := "#!/bin/sh\nexit 42"
	if err := os.WriteFile(failPath, []byte(bodyFail), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	_, err = w.executeCommand(failPath, "dummy")
	if err == nil {
		t.Fatalf("executeCommand() expected error, got nil")
	}
}

func TestProcessStaircaseWithoutSnapshots(t *testing.T) {
	migrations := []string{"1.sql", "2.sql", "3.sql"}
	tests := []struct {
		name  string
		depth int
		want  string
	}{
		{name: "all migrations", depth: 0, want: "UUUDUDDUDDUDUUU"},
		{name: "bounded depth", depth: 2, want: "UUUDUDDUDUU"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "steps")
			worker := &StaircaseWorker{
				upgradeCmd:   "printf U >> " + logPath,
				downgradeCmd: "printf D >> " + logPath,
				depth:        tt.depth,
			}

			if err := worker.processStaircase(migrations); err != nil {
				t.Fatalf("processStaircase() unexpected error: %v", err)
			}
			got, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read command log: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("command order = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcessDownUpDownRequiresBaselineWhenComparing(t *testing.T) {
	worker := &StaircaseWorker{
		compareSchemaSnapshots: true,
		baseline:               make(map[string]*driver.SchemaSnapshot),
	}

	err := worker.processDownUpDown([]string{"1.sql"})
	if !errors.Is(err, ErrBaselineNotFound()) {
		t.Fatalf("processDownUpDown() error = %v, want ErrBaselineNotFound", err)
	}
}

func TestProcessDownUpDownRequiresInitialBaselineBeforeCommand(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "steps")
	worker := &StaircaseWorker{
		compareSchemaSnapshots: true,
		upgradeCmd:             "printf U >> " + logPath,
		downgradeCmd:           "printf D >> " + logPath,
		baseline: map[string]*driver.SchemaSnapshot{
			"1.sql": {},
		},
	}

	err := worker.processDownUpDown([]string{"1.sql"})
	if !errors.Is(err, ErrBaselineNotFound()) {
		t.Fatalf("processDownUpDown() error = %v, want ErrBaselineNotFound", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("migration command ran before baseline validation: %v", err)
	}
}

func TestProcessDownUpDownRequiresPredecessorBaselineBeforeCommand(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "steps")
	worker := &StaircaseWorker{
		compareSchemaSnapshots: true,
		depth:                  1,
		upgradeCmd:             "printf U >> " + logPath,
		downgradeCmd:           "printf D >> " + logPath,
		baseline: map[string]*driver.SchemaSnapshot{
			"2.sql": {},
		},
	}

	err := worker.processDownUpDown([]string{"1.sql", "2.sql"})
	if !errors.Is(err, ErrBaselineNotFound()) {
		t.Fatalf("processDownUpDown() error = %v, want ErrBaselineNotFound", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("migration command ran before baseline validation: %v", err)
	}
}

func TestReapplyMigrationsRequiresBaselineWhenComparing(t *testing.T) {
	worker := &StaircaseWorker{
		compareSchemaSnapshots: true,
		upgradeCmd:             "true",
		baseline:               make(map[string]*driver.SchemaSnapshot),
	}

	err := worker.reapplyMigrations([]string{"1.sql"})
	if !errors.Is(err, ErrBaselineNotFound()) {
		t.Fatalf("reapplyMigrations() error = %v, want ErrBaselineNotFound", err)
	}
}

func TestCommandUnixLiteralArguments(t *testing.T) {
	command := commandHelper(t)
	for _, shell := range []string{"", "sh", "bash", "zsh"} {
		t.Run("shell="+shell, func(t *testing.T) {
			if shell != "" {
				if _, err := exec.LookPath(shell); err != nil {
					t.Skipf("shell unavailable: %v", err)
				}
			}
			t.Setenv("SHELL", shell)
			for _, migration := range unusualMigrations {
				for _, count := range []int{1, 2} {
					result := runCommandResult(t, command+" read"+strings.Repeat(` "$SEQWALL_CURRENT_MIGRATION"`, count), migration)
					want := make([]string, count)
					for i := range want {
						want[i] = migration
					}
					if !reflect.DeepEqual(result.Args, want) {
						t.Fatalf("argv = %q, want %q", result.Args, want)
					}
				}
			}
		})
	}
}

func TestCommandUnixFilenameDoesNotExecute(t *testing.T) {
	command := commandHelper(t)
	marker := filepath.Join(t.TempDir(), "marker")
	for _, migration := range []string{"$(touch " + quoteCommandPath(marker) + ").sql", "`touch " + quoteCommandPath(marker) + "`.sql"} {
		result := runCommandResult(t, command+` read "$SEQWALL_CURRENT_MIGRATION"`, migration)
		if !reflect.DeepEqual(result.Args, []string{migration}) {
			t.Fatalf("argv = %q, want literal %q", result.Args, migration)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("filename executed: marker stat error %v", err)
		}
	}
}

func TestCommandUnixLegacySingleQuotes(t *testing.T) {
	result := runCommandResult(t, commandHelper(t)+" read '{current_migration}'", "migrations/001.sql")
	if !reflect.DeepEqual(result.Args, []string{"migrations/001.sql"}) {
		t.Fatalf("argv = %q", result.Args)
	}
}
