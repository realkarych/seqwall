package seqwall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

type baselineMigration struct {
	Up   string `json:"up"`
	Down string `json:"down"`
}

func TestPostgresBaselineCommandRunner(t *testing.T) {
	if os.Getenv("SEQWALL_BASELINE_RUNNER") != "1" {
		return
	}
	direction := os.Getenv("SEQWALL_BASELINE_DIRECTION")
	migrationPath := os.Args[len(os.Args)-1]
	contents, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	var migration baselineMigration
	if err := json.Unmarshal(contents, &migration); err != nil {
		t.Fatalf("decode migration: %v", err)
	}
	query := migration.Up
	if direction == "down" {
		query = migration.Down
		if os.Getenv("SEQWALL_BASELINE_MODE") == "skip-final-down" {
			counterPath := os.Getenv("SEQWALL_BASELINE_COUNTER")
			count := 0
			if value, readErr := os.ReadFile(counterPath); readErr == nil {
				count, err = strconv.Atoi(string(value))
				if err != nil {
					t.Fatalf("parse down counter: %v", err)
				}
			} else if !os.IsNotExist(readErr) {
				t.Fatalf("read down counter: %v", readErr)
			}
			count++
			if err := os.WriteFile(counterPath, []byte(strconv.Itoa(count)), 0o600); err != nil {
				t.Fatalf("write down counter: %v", err)
			}
			if count > 1 {
				query = "SELECT 1"
			}
		}
	}
	if logPath := os.Getenv("SEQWALL_BASELINE_LOG"); logPath != "" {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatalf("open command log: %v", err)
		}
		if _, err := fmt.Fprintf(file, "%s:%s\n", direction, filepath.Base(migrationPath)); err != nil {
			_ = file.Close()
			t.Fatalf("write command log: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close command log: %v", err)
		}
	}
	client, err := driver.NewPostgresClient(os.Getenv("SEQWALL_TEST_POSTGRES_URL"))
	if err != nil {
		t.Fatalf("connect command runner: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close command runner: %v", err)
		}
	}()
	result, err := client.Execute(query)
	if err != nil {
		t.Fatalf("execute %s migration: %v", direction, err)
	}
	if result.Rows != nil {
		if err := result.Rows.Close(); err != nil {
			t.Fatalf("close command rows: %v", err)
		}
	}
}

func TestPostgresFirstMigrationRollbackUsesInitialSnapshot(t *testing.T) {
	worker, schema := newBaselineIntegrationWorker(t, 0, "", "")
	migration := writeBaselineMigration(t,
		"001_leftover.json",
		"CREATE TABLE IF NOT EXISTS "+qualifiedName(schema, "leftover")+"(id integer)",
		"SELECT 1",
	)

	err := worker.processStaircase([]string{migration})
	if err == nil || !strings.Contains(err.Error(), "snapshot after first down") {
		t.Fatalf("processStaircase() error = %v, want first down snapshot failure", err)
	}
}

func TestPostgresReversibleFirstMigrationRetainsInitialSchema(t *testing.T) {
	worker, schema := newBaselineIntegrationWorker(t, 0, "", "")
	postgresExec(t, worker, "CREATE TABLE "+qualifiedName(schema, "unrelated")+"(id integer)")
	migration := writeBaselineMigration(t,
		"001_reversible.json",
		"CREATE TABLE "+qualifiedName(schema, "created")+"(id integer)",
		"DROP TABLE "+qualifiedName(schema, "created"),
	)

	if err := worker.processStaircase([]string{migration}); err != nil {
		t.Fatalf("processStaircase() unexpected error: %v", err)
	}
	snapshot := postgresSnapshot(t, worker)
	if _, ok := snapshot.Tables[schema+".unrelated"]; !ok {
		t.Fatal("pre-existing unrelated table was not retained")
	}
	if _, ok := snapshot.Tables[schema+".created"]; !ok {
		t.Fatal("re-applied migration table was not retained")
	}
}

func TestPostgresFinalRollbackUsesInitialSnapshot(t *testing.T) {
	counterPath := filepath.Join(t.TempDir(), "down-count")
	worker, schema := newBaselineIntegrationWorker(t, 0, "skip-final-down", counterPath)
	migration := writeBaselineMigration(t,
		"001_final_down.json",
		"CREATE TABLE "+qualifiedName(schema, "final_down")+"(id integer)",
		"DROP TABLE "+qualifiedName(schema, "final_down"),
	)

	err := worker.processStaircase([]string{migration})
	if err == nil || !strings.Contains(err.Error(), "snapshot after final down") {
		t.Fatalf("processStaircase() error = %v, want final down snapshot failure", err)
	}
}

func TestPostgresStaircaseDepthSnapshots(t *testing.T) {
	for _, tt := range []struct {
		name  string
		depth int
		want  string
	}{
		{name: "bounded at migration one", depth: 1, want: "up:001_base.json\nup:002_top.json\ndown:002_top.json\nup:002_top.json\ndown:002_top.json\nup:002_top.json\n"},
		{name: "rollback to initial snapshot", depth: 0, want: "up:001_base.json\nup:002_top.json\ndown:002_top.json\nup:002_top.json\ndown:002_top.json\ndown:001_base.json\nup:001_base.json\ndown:001_base.json\nup:001_base.json\nup:002_top.json\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "commands")
			worker, schema := newBaselineIntegrationWorker(t, tt.depth, "", "")
			worker.upgradeCmd = baselineCommand("up", "", "", logPath)
			worker.downgradeCmd = baselineCommand("down", "", "", logPath)
			migrations := []string{
				writeBaselineMigration(t, "001_base.json", "CREATE TABLE "+qualifiedName(schema, "base")+"(id integer)", "DROP TABLE "+qualifiedName(schema, "base")),
				writeBaselineMigration(t, "002_top.json", "CREATE TABLE "+qualifiedName(schema, "top")+"(id integer)", "DROP TABLE "+qualifiedName(schema, "top")),
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

func TestPostgresInitialSnapshotFailurePrecedesUpgrade(t *testing.T) {
	worker, schema := newBaselineIntegrationWorker(t, 0, "", "")
	logPath := filepath.Join(t.TempDir(), "commands")
	worker.upgradeCmd = "printf U >> " + shellQuote(logPath)
	migration := writeBaselineMigration(t,
		"001_never_run.json",
		"CREATE TABLE "+qualifiedName(schema, "never_run")+"(id integer)",
		"DROP TABLE "+qualifiedName(schema, "never_run"),
	)
	closedClient, err := driver.NewPostgresClient(os.Getenv("SEQWALL_TEST_POSTGRES_URL"))
	if err != nil {
		t.Fatalf("connect PostgreSQL client to close: %v", err)
	}
	worker.dbClient = closedClient
	if err := worker.dbClient.Close(); err != nil {
		t.Fatalf("close integration PostgreSQL before snapshot: %v", err)
	}

	err = worker.processStaircase([]string{migration})
	if err == nil || !strings.Contains(err.Error(), "initial snapshot") {
		t.Fatalf("processStaircase() error = %v, want initial snapshot failure", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("upgrade command ran before initial snapshot failure: %v", err)
	}
}

func newBaselineIntegrationWorker(t *testing.T, depth int, mode, counterPath string) (*StaircaseWorker, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("baseline integration runner uses POSIX environment assignments")
	}
	worker := newPostgresIntegrationWorker(t, 1)
	worker.compareSchemaSnapshots = true
	worker.depth = depth
	worker.baseline = make(map[string]*driver.SchemaSnapshot)
	worker.upgradeCmd = baselineCommand("up", mode, counterPath, "")
	worker.downgradeCmd = baselineCommand("down", mode, counterPath, "")
	return worker, worker.schemas[0]
}

func writeBaselineMigration(t *testing.T, name, up, down string) string {
	t.Helper()
	contents, err := json.Marshal(baselineMigration{Up: up, Down: down})
	if err != nil {
		t.Fatalf("encode migration: %v", err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	return path
}

func baselineCommand(direction, mode, counterPath, logPath string) string {
	parts := []string{
		"SEQWALL_BASELINE_RUNNER=1",
		"SEQWALL_BASELINE_DIRECTION=" + shellQuote(direction),
		"SEQWALL_BASELINE_MODE=" + shellQuote(mode),
		"SEQWALL_BASELINE_COUNTER=" + shellQuote(counterPath),
		"SEQWALL_BASELINE_LOG=" + shellQuote(logPath),
		shellQuote(os.Args[0]),
		"-test.run=" + shellQuote("^TestPostgresBaselineCommandRunner$"),
		"--",
		shellQuote(CurrentMigrationPlaceholder),
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func qualifiedName(schema, relation string) string {
	return pq.QuoteIdentifier(schema) + "." + pq.QuoteIdentifier(relation)
}
