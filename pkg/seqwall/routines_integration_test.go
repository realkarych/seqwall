package seqwall

import "testing"

func TestPostgresRoutineChanges(t *testing.T) {
	tests := []struct {
		name   string
		change string
	}{
		{"body", ".answer() RETURNS integer LANGUAGE sql AS 'SELECT 2'"},
		{"volatility", ".answer() RETURNS integer LANGUAGE sql IMMUTABLE AS 'SELECT 1'"},
		{"security", ".answer() RETURNS integer LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			postgresExec(t, s, "CREATE FUNCTION "+s.schemas[0]+".answer() RETURNS integer LANGUAGE sql AS 'SELECT 1'")
			before := postgresSnapshot(t, s)
			postgresExec(t, s, "CREATE OR REPLACE FUNCTION "+s.schemas[0]+tt.change)
			after := postgresSnapshot(t, s)
			if err := compareSchemas(before, after); err == nil {
				t.Fatal("routine change was not detected")
			}
		})
	}
}

func TestPostgresRoutineOverloads(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	for _, arg := range []string{"integer", "text"} {
		postgresExec(t, s, "CREATE FUNCTION "+s.schemas[0]+".answer("+arg+") RETURNS integer LANGUAGE sql AS 'SELECT 1'")
	}
	snapshot := postgresSnapshot(t, s)
	if len(snapshot.Functions) != 2 {
		t.Fatalf("expected two overloads, got %d", len(snapshot.Functions))
	}
	for _, arg := range []string{"integer", "text"} {
		if _, ok := snapshot.Functions[s.schemas[0]+".answer("+arg+")"]; !ok {
			t.Errorf("missing %s overload", arg)
		}
	}
	postgresExec(t, s, "CREATE OR REPLACE FUNCTION "+s.schemas[0]+".answer(integer) RETURNS integer LANGUAGE sql AS 'SELECT 2'")
	if err := compareSchemas(snapshot, postgresSnapshot(t, s)); err == nil {
		t.Fatal("overload body change was not detected")
	}
}

func TestPostgresRoutineSchemas(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	for _, schema := range s.schemas {
		postgresExec(t, s, "CREATE FUNCTION "+schema+".answer() RETURNS integer LANGUAGE sql AS 'SELECT 1'")
	}
	snapshot := postgresSnapshot(t, s)
	if len(snapshot.Functions) != 2 {
		t.Fatalf("expected two schema-qualified routines, got %d", len(snapshot.Functions))
	}
	for _, schema := range s.schemas {
		if _, ok := snapshot.Functions[schema+".answer()"]; !ok {
			t.Errorf("missing routine in schema %s", schema)
		}
	}
}

func TestPostgresRoutineRecreation(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	query := "CREATE FUNCTION " + s.schemas[0] + ".answer(value integer DEFAULT 1) RETURNS integer LANGUAGE sql IMMUTABLE AS 'SELECT value'"
	postgresExec(t, s, query)
	before := postgresSnapshot(t, s)
	postgresExec(t, s, "DROP FUNCTION "+s.schemas[0]+".answer(integer)")
	postgresExec(t, s, query)
	if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("equivalent routine recreation differs: %v", err)
	}
}

func TestPostgresRoutineProcedure(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	query := "CREATE PROCEDURE " + s.schemas[0] + ".answer() LANGUAGE sql AS 'SELECT 1'"
	postgresExec(t, s, query)
	before := postgresSnapshot(t, s)
	procedure, ok := before.Functions[s.schemas[0]+".answer()"]
	if !ok || procedure.RoutineName != "answer" || procedure.RoutineType != "PROCEDURE" || procedure.ReturnType != "" {
		t.Fatalf("unexpected procedure snapshot: %+v", before.Functions)
	}
	postgresExec(t, s, "DROP PROCEDURE "+s.schemas[0]+".answer()")
	postgresExec(t, s, query)
	if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("equivalent procedure recreation differs: %v", err)
	}
	postgresExec(t, s, "CREATE OR REPLACE PROCEDURE "+s.schemas[0]+".answer() LANGUAGE sql AS 'SELECT 2'")
	if err := compareSchemas(before, postgresSnapshot(t, s)); err == nil {
		t.Fatal("procedure body change was not detected")
	}
}

func TestPostgresRoutineExcludesAggregates(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	postgresExec(t, s, "CREATE AGGREGATE "+s.schemas[0]+".total(integer) (SFUNC = pg_catalog.int4pl, STYPE = integer)")
	snapshot := postgresSnapshot(t, s)
	if len(snapshot.Functions) != 0 {
		t.Fatalf("aggregate included in routine snapshots: %+v", snapshot.Functions)
	}
}
