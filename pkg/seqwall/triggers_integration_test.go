package seqwall

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

func TestPostgresTriggerChanges(t *testing.T) {
	tests := []struct {
		name      string
		before    string
		after     string
		wantAfter string
	}{
		{
			name:      "events",
			before:    "CREATE TRIGGER tr BEFORE INSERT OR UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			after:     "CREATE TRIGGER tr BEFORE UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			wantAfter: "BEFORE UPDATE",
		},
		{
			name:      "orientation",
			before:    "CREATE TRIGGER tr BEFORE UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			after:     "CREATE TRIGGER tr BEFORE UPDATE ON %s FOR EACH STATEMENT EXECUTE FUNCTION %s()",
			wantAfter: "FOR EACH STATEMENT",
		},
		{
			name:      "condition",
			before:    "CREATE TRIGGER tr BEFORE UPDATE ON %s FOR EACH ROW WHEN (NEW.x > 0) EXECUTE FUNCTION %s()",
			after:     "CREATE TRIGGER tr BEFORE UPDATE ON %s FOR EACH ROW WHEN (NEW.x > 1) EXECUTE FUNCTION %s()",
			wantAfter: "new.x > 1",
		},
		{
			name:      "updated columns",
			before:    "CREATE TRIGGER tr BEFORE UPDATE OF x ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			after:     "CREATE TRIGGER tr BEFORE UPDATE OF y ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			wantAfter: "UPDATE OF y",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			schema := pq.QuoteIdentifier(s.schemas[0])
			table := schema + ".items"
			function := schema + ".trigger_fn"
			postgresExec(t, s, "CREATE FUNCTION "+function+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$")
			postgresExec(t, s, "CREATE TABLE "+table+" (x integer, y integer)")
			postgresExec(t, s, fmt.Sprintf(test.before, table, function))

			before := postgresSnapshot(t, s)
			key := s.schemas[0] + ".items.tr"
			assertTriggerMetadata(t, before.Triggers[key], s.schemas[0], "items", "O", "CREATE TRIGGER tr")

			postgresExec(t, s, "DROP TRIGGER tr ON "+table)
			postgresExec(t, s, fmt.Sprintf(test.after, table, function))
			after := postgresSnapshot(t, s)
			assertTriggerMetadata(t, after.Triggers[key], s.schemas[0], "items", "O", test.wantAfter)
			assertSnapshotsDiffer(t, before, after)

			postgresExec(t, s, "DROP TRIGGER tr ON "+table)
			postgresExec(t, s, fmt.Sprintf(test.before, table, function))
			if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
				t.Fatalf("restoring original trigger changed snapshot: %v", err)
			}
		})
	}
}

func TestPostgresTriggerEnabledState(t *testing.T) {
	tests := []struct {
		name    string
		alter   string
		enabled string
	}{
		{name: "disabled", alter: "DISABLE TRIGGER tr", enabled: "D"},
		{name: "replica", alter: "ENABLE REPLICA TRIGGER tr", enabled: "R"},
		{name: "always", alter: "ENABLE ALWAYS TRIGGER tr", enabled: "A"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			schema := pq.QuoteIdentifier(s.schemas[0])
			table := schema + ".items"
			function := schema + ".trigger_fn"
			postgresExec(t, s, "CREATE FUNCTION "+function+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$")
			postgresExec(t, s, "CREATE TABLE "+table+" (x integer, y integer)")
			postgresExec(t, s, "CREATE TRIGGER tr BEFORE UPDATE ON "+table+" FOR EACH ROW EXECUTE FUNCTION "+function+"()")

			before := postgresSnapshot(t, s)
			key := s.schemas[0] + ".items.tr"
			assertTriggerMetadata(t, before.Triggers[key], s.schemas[0], "items", "O", "CREATE TRIGGER tr")

			postgresExec(t, s, "ALTER TABLE "+table+" "+test.alter)
			after := postgresSnapshot(t, s)
			assertTriggerMetadata(t, after.Triggers[key], s.schemas[0], "items", test.enabled, "CREATE TRIGGER tr")
			assertSnapshotsDiffer(t, before, after)

			postgresExec(t, s, "ALTER TABLE "+table+" ENABLE TRIGGER tr")
			if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
				t.Fatalf("restoring normal trigger enablement changed snapshot: %v", err)
			}
		})
	}
}

func TestPostgresTriggerQualifiedIdentityAndInternalExclusion(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	schema1 := pq.QuoteIdentifier(s.schemas[0])
	schema2 := pq.QuoteIdentifier(s.schemas[1])
	function1 := schema1 + ".trigger_fn"
	function2 := schema2 + ".trigger_fn"
	postgresExec(t, s, "CREATE FUNCTION "+function1+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$")
	postgresExec(t, s, "CREATE FUNCTION "+function2+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$")
	postgresExec(t, s, "CREATE TABLE "+schema1+".items (x integer)")
	postgresExec(t, s, "CREATE TABLE "+schema1+".other (x integer)")
	postgresExec(t, s, "CREATE TABLE "+schema2+".items (x integer)")
	postgresExec(t, s, "CREATE TRIGGER tr BEFORE UPDATE ON "+schema1+".items FOR EACH ROW EXECUTE FUNCTION "+function1+"()")
	postgresExec(t, s, "CREATE TRIGGER tr BEFORE UPDATE ON "+schema1+".other FOR EACH ROW EXECUTE FUNCTION "+function1+"()")
	postgresExec(t, s, "CREATE TRIGGER tr BEFORE UPDATE ON "+schema2+".items FOR EACH ROW EXECUTE FUNCTION "+function2+"()")

	before := postgresSnapshot(t, s)
	triggers := []struct {
		key    string
		schema string
		table  string
	}{
		{key: s.schemas[0] + ".items.tr", schema: s.schemas[0], table: "items"},
		{key: s.schemas[0] + ".other.tr", schema: s.schemas[0], table: "other"},
		{key: s.schemas[1] + ".items.tr", schema: s.schemas[1], table: "items"},
	}
	if len(before.Triggers) != len(triggers) {
		t.Fatalf("trigger count = %d, want %d: %+v", len(before.Triggers), len(triggers), before.Triggers)
	}
	for _, trigger := range triggers {
		assertTriggerMetadata(t, before.Triggers[trigger.key], trigger.schema, trigger.table, "O", "CREATE TRIGGER tr")
	}

	postgresExec(t, s, "DROP TRIGGER tr ON "+schema1+".items")
	postgresExec(t, s, "CREATE TRIGGER tr BEFORE INSERT ON "+schema1+".items FOR EACH ROW EXECUTE FUNCTION "+function1+"()")
	after := postgresSnapshot(t, s)
	if reflect.DeepEqual(before.Triggers[triggers[0].key], after.Triggers[triggers[0].key]) {
		t.Fatalf("modified trigger %q did not change: before=%+v after=%+v", triggers[0].key, before.Triggers[triggers[0].key], after.Triggers[triggers[0].key])
	}
	for _, trigger := range triggers[1:] {
		if !reflect.DeepEqual(before.Triggers[trigger.key], after.Triggers[trigger.key]) {
			t.Fatalf("unmodified trigger %q changed: before=%+v after=%+v", trigger.key, before.Triggers[trigger.key], after.Triggers[trigger.key])
		}
	}

	postgresExec(t, s, "DROP TRIGGER tr ON "+schema1+".items")
	postgresExec(t, s, "CREATE TRIGGER tr BEFORE UPDATE ON "+schema1+".items FOR EACH ROW EXECUTE FUNCTION "+function1+"()")
	if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("equivalent trigger recreation changed snapshot: %v", err)
	}

	postgresExec(t, s, "CREATE TABLE "+schema1+".parent (id integer PRIMARY KEY)")
	postgresExec(t, s, "CREATE TABLE "+schema1+".child (parent_id integer REFERENCES "+schema1+".parent (id))")
	withForeignKey := postgresSnapshot(t, s)
	if len(withForeignKey.Triggers) != len(triggers) {
		t.Fatalf("trigger count with foreign key = %d, want %d user triggers: %+v", len(withForeignKey.Triggers), len(triggers), withForeignKey.Triggers)
	}
}

func TestPostgresTriggerUserConstraintTrigger(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	table := schema + ".items"
	function := schema + ".trigger_fn"
	postgresExec(t, s, "CREATE FUNCTION "+function+"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$")
	postgresExec(t, s, "CREATE TABLE "+table+" (x integer, y integer)")
	postgresExec(t, s, "CREATE CONSTRAINT TRIGGER tr AFTER INSERT ON "+table+" DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION "+function+"()")

	before := postgresSnapshot(t, s)
	key := s.schemas[0] + ".items.tr"
	if len(before.Triggers) != 1 {
		t.Fatalf("constraint trigger count = %d, want 1: %+v", len(before.Triggers), before.Triggers)
	}
	assertTriggerMetadata(t, before.Triggers[key], s.schemas[0], "items", "O", "DEFERRABLE INITIALLY DEFERRED")

	postgresExec(t, s, "DROP TRIGGER tr ON "+table)
	postgresExec(t, s, "CREATE CONSTRAINT TRIGGER tr AFTER INSERT ON "+table+" DEFERRABLE INITIALLY IMMEDIATE FOR EACH ROW EXECUTE FUNCTION "+function+"()")
	after := postgresSnapshot(t, s)
	assertTriggerMetadata(t, after.Triggers[key], s.schemas[0], "items", "O", "DEFERRABLE INITIALLY IMMEDIATE")
	assertSnapshotsDiffer(t, before, after)
}

func assertTriggerMetadata(t *testing.T, trigger driver.TriggerDefinition, schema, table, enabled, definitionPart string) {
	t.Helper()
	encoded, err := json.Marshal(trigger)
	if err != nil {
		t.Fatalf("marshal trigger metadata: %v", err)
	}
	var fields map[string]string
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal trigger metadata: %v", err)
	}
	want := map[string]string{
		"trigger_name": "tr",
		"table_schema": schema,
		"table_name":   table,
		"definition":   fields["definition"],
		"enabled":      enabled,
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("trigger metadata = %+v, want fields %+v", fields, want)
	}
	if !strings.Contains(fields["definition"], definitionPart) {
		t.Fatalf("trigger definition = %q, want substring %q", fields["definition"], definitionPart)
	}
}
