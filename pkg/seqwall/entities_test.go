package seqwall

import (
	"reflect"
	"testing"
)

func TestNewStaircaseWorker_InitializesFields(t *testing.T) {
	migrationsPath := "./migrations"
	compareSnapshots := true
	depth := 5
	upgradeCmd := "up_cmd"
	downgradeCmd := "down_cmd"
	postgresURL := "postgres://user:pass@localhost/db"
	schemas := []string{"public", "audit"}
	migrationsExt := ".sql"

	worker := NewStaircaseWorker(
		migrationsPath,
		compareSnapshots,
		depth,
		upgradeCmd,
		downgradeCmd,
		postgresURL,
		schemas,
		migrationsExt,
	)
	if worker == nil {
		t.Fatal("expected NewStaircaseWorker to return a non-nil worker")
	}
	if worker.migrationsPath != migrationsPath {
		t.Errorf("migrationsPath = %q; want %q", worker.migrationsPath, migrationsPath)
	}
	if worker.compareSchemaSnapshots != compareSnapshots {
		t.Errorf("compareSchemaSnapshots = %v; want %v", worker.compareSchemaSnapshots, compareSnapshots)
	}
	if worker.depth != depth {
		t.Errorf("depth = %d; want %d", worker.depth, depth)
	}
	if worker.upgradeCmd != upgradeCmd {
		t.Errorf("upgradeCmd = %q; want %q", worker.upgradeCmd, upgradeCmd)
	}
	if worker.downgradeCmd != downgradeCmd {
		t.Errorf("downgradeCmd = %q; want %q", worker.downgradeCmd, downgradeCmd)
	}
	if worker.postgresURL != postgresURL {
		t.Errorf("postgresURL = %q; want %q", worker.postgresURL, postgresURL)
	}
	if !reflect.DeepEqual(worker.schemas, schemas) {
		t.Errorf("schemas = %v; want %v", worker.schemas, schemas)
	}
	if worker.migrationsExtension != migrationsExt {
		t.Errorf("migrationsExtension = %q; want %q", worker.migrationsExtension, migrationsExt)
	}
	if worker.dbClient != nil {
		t.Error("expected dbClient to be nil on initialization")
	}
	if worker.baseline == nil {
		t.Error("expected baseline map to be non-nil")
	}
	if len(worker.baseline) != 0 {
		t.Errorf("expected baseline map to be empty, got %d entries", len(worker.baseline))
	}
}
