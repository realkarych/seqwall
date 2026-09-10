package seqwall

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

func TestPostgresSnapshotTableSchemaMove(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	schema1 := pq.QuoteIdentifier(s.schemas[0])
	schema2 := pq.QuoteIdentifier(s.schemas[1])
	postgresExec(t, s, "CREATE TABLE "+schema1+".t (x integer)")

	before := postgresSnapshot(t, s)
	postgresExec(t, s, "ALTER TABLE "+schema1+".t SET SCHEMA "+schema2)
	after := postgresSnapshot(t, s)
	if err := compareSchemas(before, after); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("schema move comparison error = %v, want ErrSnapshotsDiffer", err)
	}
}

func TestPostgresSnapshotSameNamedTables(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	schema1 := pq.QuoteIdentifier(s.schemas[0])
	schema2 := pq.QuoteIdentifier(s.schemas[1])
	postgresExec(t, s, "CREATE TABLE "+schema1+".items (first integer, second text)")
	postgresExec(t, s, "CREATE TABLE "+schema2+".items (other boolean)")

	before := postgresSnapshot(t, s)
	if len(before.Tables) != 2 {
		t.Fatalf("table count = %d, want 2: %+v", len(before.Tables), before.Tables)
	}
	assertSnapshotColumns(t, before.Tables, s.schemas[0]+".items", "first", "second")
	assertSnapshotColumns(t, before.Tables, s.schemas[1]+".items", "other")

	postgresExec(t, s, "DROP TABLE "+schema1+".items")
	if err := compareSchemas(before, postgresSnapshot(t, s)); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("dropping %s.items comparison error = %v, want ErrSnapshotsDiffer", s.schemas[0], err)
	}
	postgresExec(t, s, "CREATE TABLE "+schema1+".items (first integer, second text)")
	postgresExec(t, s, "DROP TABLE "+schema2+".items")
	if err := compareSchemas(before, postgresSnapshot(t, s)); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("dropping %s.items comparison error = %v, want ErrSnapshotsDiffer", s.schemas[1], err)
	}
}

func TestPostgresSnapshotQualifiedEmptyTables(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	for _, schema := range s.schemas {
		postgresExec(t, s, "CREATE TABLE "+pq.QuoteIdentifier(schema)+".empty ()")
	}

	snapshot := postgresSnapshot(t, s)
	if len(snapshot.Tables) != 2 {
		t.Fatalf("empty table count = %d, want 2: %+v", len(snapshot.Tables), snapshot.Tables)
	}
	for _, schema := range s.schemas {
		if _, ok := snapshot.Tables[schema+".empty"]; !ok {
			t.Fatalf("qualified empty table %q missing from snapshot", schema+".empty")
		}
	}
}

func TestPostgresSnapshotSameNamedObjects(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	schema1 := pq.QuoteIdentifier(s.schemas[0])
	schema2 := pq.QuoteIdentifier(s.schemas[1])
	postgresExec(t, s, "CREATE TYPE "+schema1+".state AS ENUM ('new', 'done')")
	postgresExec(t, s, "CREATE TYPE "+schema2+".state AS ENUM ('queued', 'sent', 'read')")
	postgresExec(t, s, "CREATE TABLE "+schema1+".source (first integer)")
	postgresExec(t, s, "CREATE TABLE "+schema2+".source (other integer)")
	postgresExec(t, s, "CREATE INDEX same_index ON "+schema1+".source (first)")
	postgresExec(t, s, "CREATE INDEX same_index ON "+schema2+".source (other)")
	postgresExec(t, s, "CREATE VIEW "+schema1+".same_view AS SELECT 11 AS value")
	postgresExec(t, s, "CREATE VIEW "+schema2+".same_view AS SELECT 22 AS value")
	postgresExec(t, s, "CREATE MATERIALIZED VIEW "+schema1+".same_matview AS SELECT 33 AS value")
	postgresExec(t, s, "CREATE MATERIALIZED VIEW "+schema2+".same_matview AS SELECT 44 AS value")
	postgresExec(t, s, "CREATE SEQUENCE "+schema1+".same_sequence START 3")
	postgresExec(t, s, "CREATE SEQUENCE "+schema2+".same_sequence START 7")

	snapshot := postgresSnapshot(t, s)
	if len(snapshot.EnumTypes) != 2 {
		t.Fatalf("enum count = %d, want 2: %+v", len(snapshot.EnumTypes), snapshot.EnumTypes)
	}
	if got := snapshot.EnumTypes[s.schemas[0]+".state"].Labels; !reflect.DeepEqual(got, []string{"new", "done"}) {
		t.Fatalf("first enum labels = %v, want [new done]", got)
	}
	if got := snapshot.EnumTypes[s.schemas[1]+".state"].Labels; !reflect.DeepEqual(got, []string{"queued", "sent", "read"}) {
		t.Fatalf("second enum labels = %v, want [queued sent read]", got)
	}
	assertTwoQualifiedDefinitions(t, snapshot.Views, s.schemas, "same_view", func(def driver.ViewDefinition) string { return def.Definition }, "11", "22")
	assertTwoQualifiedDefinitions(t, snapshot.MatViews, s.schemas, "same_matview", func(def driver.MatViewDefinition) string { return def.Definition }, "33", "44")
	assertTwoQualifiedDefinitions(t, snapshot.Indexes, s.schemas, "same_index", func(def driver.IndexDefinition) string { return def.IndexDef }, "first", "other")
	if len(snapshot.Sequences) != 2 {
		t.Fatalf("sequence count = %d, want 2: %+v", len(snapshot.Sequences), snapshot.Sequences)
	}
	if got := snapshot.Sequences[s.schemas[0]+".same_sequence"].StartValue; got != "3" {
		t.Fatalf("first sequence start = %q, want 3", got)
	}
	if got := snapshot.Sequences[s.schemas[1]+".same_sequence"].StartValue; got != "7" {
		t.Fatalf("second sequence start = %q, want 7", got)
	}
	if err := compareSchemas(snapshot, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("repeated snapshot changed: %v", err)
	}
}

func TestPostgresSnapshotQualifiedConstraintAndTriggerIdentities(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	baseSchema := s.schemas[0]
	dottedSchema := baseSchema + ".a"
	postgresExec(t, s, "CREATE SCHEMA "+pq.QuoteIdentifier(dottedSchema))
	t.Cleanup(func() {
		if _, err := s.dbClient.Execute("DROP SCHEMA " + pq.QuoteIdentifier(dottedSchema) + " CASCADE"); err != nil {
			t.Errorf("drop integration schema %q: %v", dottedSchema, err)
		}
	})
	s.schemas = append(s.schemas, dottedSchema)

	for _, schema := range s.schemas {
		qualifiedSchema := pq.QuoteIdentifier(schema)
		postgresExec(t, s, "CREATE FUNCTION "+qualifiedSchema+".trigger_fn() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END'")
		for i, tableName := range []string{"a.b", "b"} {
			qualifiedTable := qualifiedSchema + "." + pq.QuoteIdentifier(tableName)
			postgresExec(t, s, fmt.Sprintf("CREATE TABLE %s (x integer CONSTRAINT ck CHECK (x > %d))", qualifiedTable, i))
			postgresExec(t, s, "CREATE TRIGGER tr BEFORE INSERT ON "+qualifiedTable+" FOR EACH ROW EXECUTE FUNCTION "+qualifiedSchema+".trigger_fn()")
		}
	}

	snapshot := &driver.SchemaSnapshot{Constraints: make(map[string]driver.ConstraintDefinition)}
	if err := s.scanConstraints(snapshot); err != nil {
		t.Fatalf("scan constraints: %v", err)
	}
	if err := s.scanTriggers(snapshot); err != nil {
		t.Fatalf("scan triggers: %v", err)
	}
	if len(snapshot.Constraints) != 4 {
		t.Fatalf("constraint count = %d, want 4: %+v", len(snapshot.Constraints), snapshot.Constraints)
	}
	if len(snapshot.Triggers) != 4 {
		t.Fatalf("trigger count = %d, want 4: %+v", len(snapshot.Triggers), snapshot.Triggers)
	}
	wantConstraintKeys := []string{
		baseSchema + ".\"a.b\".ck",
		baseSchema + ".b.ck",
		"\"" + dottedSchema + "\".\"a.b\".ck",
		"\"" + dottedSchema + "\".b.ck",
	}
	wantTriggerKeys := []string{
		baseSchema + ".\"a.b\".tr",
		baseSchema + ".b.tr",
		"\"" + dottedSchema + "\".\"a.b\".tr",
		"\"" + dottedSchema + "\".b.tr",
	}
	assertSnapshotKeys(t, snapshot.Constraints, wantConstraintKeys)
	assertSnapshotKeys(t, snapshot.Triggers, wantTriggerKeys)
}

func TestPostgresSnapshotPrivilegeQualifiedTables(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	for _, schema := range s.schemas {
		qualifiedTable := pq.QuoteIdentifier(schema) + ".granted"
		postgresExec(t, s, "CREATE TABLE "+qualifiedTable+" (id integer)")
		postgresExec(t, s, "GRANT SELECT ON "+qualifiedTable+" TO PUBLIC")
	}

	before := postgresSnapshot(t, s)
	publicSelect := publicSelectPrivileges(before.Privileges, "granted")
	if len(publicSelect) != 2 {
		t.Fatalf("PUBLIC SELECT privilege count = %d, want 2: %+v", len(publicSelect), publicSelect)
	}
	wantSchemas := slices.Clone(s.schemas)
	slices.Sort(wantSchemas)
	for i, schema := range wantSchemas {
		if got := publicSelect[i].TableSchema; got != schema {
			t.Fatalf("PUBLIC SELECT privilege %d schema = %q, want %q", i, got, schema)
		}
	}
	encoded, err := json.Marshal(publicSelect)
	if err != nil {
		t.Fatalf("marshal PUBLIC SELECT privileges: %v", err)
	}
	for _, schema := range wantSchemas {
		if !strings.Contains(string(encoded), `"table_schema":"`+schema+`"`) {
			t.Fatalf("serialized privileges %s do not contain table schema %q", encoded, schema)
		}
	}
	if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("repeated privilege snapshot changed: %v", err)
	}

	postgresExec(t, s, "REVOKE SELECT ON "+pq.QuoteIdentifier(s.schemas[0])+".granted FROM PUBLIC")
	after := postgresSnapshot(t, s)
	if err := compareSchemas(before, after); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("grant change comparison error = %v, want ErrSnapshotsDiffer", err)
	}
	remaining := publicSelectPrivileges(after.Privileges, "granted")
	if len(remaining) != 1 {
		t.Fatalf("remaining PUBLIC SELECT privilege count = %d, want 1: %+v", len(remaining), remaining)
	}
	encoded, err = json.Marshal(remaining)
	if err != nil {
		t.Fatalf("marshal remaining PUBLIC SELECT privilege: %v", err)
	}
	if !strings.Contains(string(encoded), `"table_schema":"`+s.schemas[1]+`"`) {
		t.Fatalf("remaining serialized privilege %s does not identify schema %q", encoded, s.schemas[1])
	}
}

func assertSnapshotColumns(t *testing.T, tables map[string]driver.TableDefinition, key string, want ...string) {
	t.Helper()
	table, ok := tables[key]
	if !ok {
		t.Fatalf("table %q missing from snapshot", key)
	}
	got := make([]string, len(table.Columns))
	for i := range table.Columns {
		got[i] = table.Columns[i].ColumnName
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("table %q columns = %v, want %v", key, got, want)
	}
}

func assertTwoQualifiedDefinitions[T any](t *testing.T, definitions map[string]T, schemas []string, name string, value func(T) string, want1, want2 string) {
	t.Helper()
	if len(definitions) != 2 {
		t.Fatalf("%s definition count = %d, want 2: %+v", name, len(definitions), definitions)
	}
	got1, ok := definitions[schemas[0]+"."+name]
	if !ok || !strings.Contains(value(got1), want1) {
		t.Fatalf("first %s definition = %+v, want value containing %q", name, got1, want1)
	}
	got2, ok := definitions[schemas[1]+"."+name]
	if !ok || !strings.Contains(value(got2), want2) {
		t.Fatalf("second %s definition = %+v, want value containing %q", name, got2, want2)
	}
}

func assertSnapshotKeys[T any](t *testing.T, values map[string]T, want []string) {
	t.Helper()
	for _, key := range want {
		if _, ok := values[key]; !ok {
			t.Errorf("snapshot key %q missing from %+v", key, values)
		}
	}
}

func publicSelectPrivileges(privileges []driver.PrivilegeDefinition, tableName string) []driver.PrivilegeDefinition {
	var matched []driver.PrivilegeDefinition
	for _, privilege := range privileges {
		if privilege.Grantee == "PUBLIC" && privilege.TableName == tableName && privilege.Privilege == "SELECT" {
			matched = append(matched, privilege)
		}
	}
	return matched
}
