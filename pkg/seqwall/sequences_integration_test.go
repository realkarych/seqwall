package seqwall

import (
	"database/sql"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

func TestPostgresSequenceChanges(t *testing.T) {
	tests := []struct {
		name   string
		alter  string
		assert func(*testing.T, driver.SequenceDefinition)
	}{
		{
			name:  "cache",
			alter: "CACHE 7",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.CacheSize != "7" {
					t.Fatalf("cache size = %q, want 7", got.CacheSize)
				}
			},
		},
		{
			name:  "increment",
			alter: "INCREMENT BY 3",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.Increment != "3" {
					t.Fatalf("increment = %q, want 3", got.Increment)
				}
			},
		},
		{
			name:  "start",
			alter: "START WITH 5",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.StartValue != "5" {
					t.Fatalf("start value = %q, want 5", got.StartValue)
				}
			},
		},
		{
			name:  "minimum",
			alter: "MINVALUE 2",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.MinValue != "2" {
					t.Fatalf("minimum value = %q, want 2", got.MinValue)
				}
			},
		},
		{
			name:  "maximum",
			alter: "MAXVALUE 900",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.MaxValue != "900" {
					t.Fatalf("maximum value = %q, want 900", got.MaxValue)
				}
			},
		},
		{
			name:  "cycle",
			alter: "CYCLE",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.CycleOption != "YES" {
					t.Fatalf("cycle option = %q, want YES", got.CycleOption)
				}
			},
		},
		{
			name:  "data type",
			alter: "AS bigint",
			assert: func(t *testing.T, got driver.SequenceDefinition) {
				if got.DataType != "bigint" {
					t.Fatalf("data type = %q, want bigint", got.DataType)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			schema := pq.QuoteIdentifier(s.schemas[0])
			sequence := schema + ".counter"
			postgresExec(t, s, "CREATE SEQUENCE "+sequence+" AS integer START WITH 3 INCREMENT BY 2 MINVALUE 1 MAXVALUE 1000 CACHE 1 NO CYCLE")
			before := postgresSnapshot(t, s)
			key := s.schemas[0] + ".counter"
			assertSequence(t, snapshotSequence(t, before, key), driver.SequenceDefinition{
				SequenceName: "counter",
				DataType:     "integer",
				StartValue:   "3",
				MinValue:     "1",
				MaxValue:     "1000",
				Increment:    "2",
				CycleOption:  "NO",
				CacheSize:    "1",
			})
			postgresExec(t, s, "ALTER SEQUENCE "+sequence+" "+test.alter)
			after := postgresSnapshot(t, s)
			test.assert(t, snapshotSequence(t, after, key))
			assertSnapshotsDiffer(t, before, after)
		})
	}
}

func TestPostgresSequenceIdentityConfigurationAndRecreation(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	table := schema + ".items"
	create := "CREATE TABLE " + table + " (id integer GENERATED ALWAYS AS IDENTITY)"
	key := s.schemas[0] + ".items_id_seq"
	postgresExec(t, s, create)

	baseline := postgresSnapshot(t, s)
	assertSequenceOwnership(t, snapshotSequence(t, baseline, key), s.schemas[0], "items", "id", "i")
	postgresExec(t, s, "ALTER TABLE "+table+" ALTER COLUMN id SET INCREMENT BY 7")
	incremented := postgresSnapshot(t, s)
	if got := snapshotSequence(t, incremented, key).Increment; got != "7" {
		t.Fatalf("identity increment = %q, want 7", got)
	}
	assertSnapshotsDiffer(t, baseline, incremented)

	postgresExec(t, s, "ALTER TABLE "+table+" ALTER COLUMN id SET INCREMENT BY 1")
	postgresExec(t, s, "ALTER TABLE "+table+" ALTER COLUMN id SET CACHE 7")
	cached := postgresSnapshot(t, s)
	if got := snapshotSequence(t, cached, key).CacheSize; got != "7" {
		t.Fatalf("identity cache size = %q, want 7", got)
	}
	assertSnapshotsDiffer(t, baseline, cached)

	postgresExec(t, s, "DROP TABLE "+table)
	postgresExec(t, s, create)
	if err := compareSchemas(baseline, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("equivalent identity table recreation changed snapshot: %v", err)
	}
}

func TestPostgresSequenceOwnership(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	postgresExec(t, s, "CREATE SEQUENCE "+schema+".counter")
	postgresExec(t, s, "CREATE TABLE "+schema+".items (x serial, y integer, id integer GENERATED ALWAYS AS IDENTITY)")

	snapshot := postgresSnapshot(t, s)
	if len(snapshot.Sequences) != 3 {
		t.Fatalf("sequence count = %d, want 3: %+v", len(snapshot.Sequences), snapshot.Sequences)
	}
	assertSequenceOwnership(t, snapshotSequence(t, snapshot, s.schemas[0]+".counter"), "", "", "", "")
	assertSequenceOwnership(t, snapshotSequence(t, snapshot, s.schemas[0]+".items_x_seq"), s.schemas[0], "items", "x", "a")
	assertSequenceOwnership(t, snapshotSequence(t, snapshot, s.schemas[0]+".items_id_seq"), s.schemas[0], "items", "id", "i")

	key := s.schemas[0] + ".counter"
	postgresExec(t, s, "ALTER SEQUENCE "+schema+".counter OWNED BY "+schema+".items.x")
	ownedByX := postgresSnapshot(t, s)
	assertSequenceOwnership(t, snapshotSequence(t, ownedByX, key), s.schemas[0], "items", "x", "a")
	assertSnapshotsDiffer(t, snapshot, ownedByX)

	postgresExec(t, s, "ALTER SEQUENCE "+schema+".counter OWNED BY "+schema+".items.y")
	ownedByY := postgresSnapshot(t, s)
	assertSequenceOwnership(t, snapshotSequence(t, ownedByY, key), s.schemas[0], "items", "y", "a")
	assertSnapshotsDiffer(t, ownedByX, ownedByY)

	postgresExec(t, s, "ALTER SEQUENCE "+schema+".counter OWNED BY NONE")
	unowned := postgresSnapshot(t, s)
	assertSequenceOwnership(t, snapshotSequence(t, unowned, key), "", "", "", "")
	assertSnapshotsDiffer(t, ownedByY, unowned)
}

func TestPostgresSequenceOwnershipWithSeparatorNames(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	tableName := "owned.table"
	columnName := "target.column"
	postgresExec(t, s, "CREATE TABLE "+schema+"."+pq.QuoteIdentifier(tableName)+" ("+pq.QuoteIdentifier(columnName)+" integer)")
	postgresExec(t, s, "CREATE SEQUENCE "+schema+".counter OWNED BY "+schema+"."+pq.QuoteIdentifier(tableName)+"."+pq.QuoteIdentifier(columnName))

	snapshot := postgresSnapshot(t, s)
	assertSequenceOwnership(t, snapshotSequence(t, snapshot, s.schemas[0]+".counter"), s.schemas[0], tableName, columnName, "a")
}

func TestPostgresSequenceRuntimeCountersIgnored(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	sequence := schema + ".counter"
	postgresExec(t, s, "CREATE SEQUENCE "+sequence+" START WITH 3")
	postgresExec(t, s, "CREATE TABLE "+schema+".items (id integer GENERATED ALWAYS AS IDENTITY)")
	baseline := postgresSnapshot(t, s)

	postgresSelectAndClose(t, s, "SELECT pg_catalog.nextval("+pq.QuoteLiteral(sequence)+"::regclass)")
	if err := compareSchemas(baseline, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("nextval changed snapshot: %v", err)
	}
	postgresSelectAndClose(t, s, "SELECT pg_catalog.setval("+pq.QuoteLiteral(sequence)+"::regclass, 40, true)")
	if err := compareSchemas(baseline, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("setval changed snapshot: %v", err)
	}
	postgresExec(t, s, "ALTER SEQUENCE "+sequence+" RESTART WITH 50")
	if err := compareSchemas(baseline, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("restart changed snapshot: %v", err)
	}
	postgresExec(t, s, "INSERT INTO "+schema+".items DEFAULT VALUES")
	if err := compareSchemas(baseline, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("identity insert changed snapshot: %v", err)
	}
}

func postgresSelectAndClose(t *testing.T, s *StaircaseWorker, query string) {
	t.Helper()
	result, err := s.dbClient.Execute(query)
	if err != nil {
		t.Fatalf("execute integration SELECT: %v", err)
	}
	if result.Rows == nil {
		t.Fatal("integration SELECT returned no rows handle")
	}
	if err := result.Rows.Close(); err != nil {
		t.Fatalf("close integration SELECT rows: %v", err)
	}
}

func assertSequence(t *testing.T, got, want driver.SequenceDefinition) {
	t.Helper()
	if got != want {
		t.Fatalf("sequence = %+v, want %+v", got, want)
	}
}

func snapshotSequence(t *testing.T, snapshot *driver.SchemaSnapshot, key string) driver.SequenceDefinition {
	t.Helper()
	sequence, ok := snapshot.Sequences[key]
	if !ok {
		t.Fatalf("sequence %q missing from %+v", key, snapshot.Sequences)
	}
	return sequence
}

func assertSequenceOwnership(t *testing.T, got driver.SequenceDefinition, schema, table, column, ownershipType string) {
	t.Helper()
	wantSchema := sql.NullString{String: schema, Valid: schema != ""}
	wantTable := sql.NullString{String: table, Valid: table != ""}
	wantColumn := sql.NullString{String: column, Valid: column != ""}
	wantType := sql.NullString{String: ownershipType, Valid: ownershipType != ""}
	if got.OwnedBySchema != wantSchema || got.OwnedByTable != wantTable || got.OwnedByColumn != wantColumn || got.OwnershipType != wantType {
		t.Fatalf("sequence ownership = schema:%+v table:%+v column:%+v type:%+v, want schema:%+v table:%+v column:%+v type:%+v", got.OwnedBySchema, got.OwnedByTable, got.OwnedByColumn, got.OwnershipType, wantSchema, wantTable, wantColumn, wantType)
	}
}
