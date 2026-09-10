package seqwall

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

func TestPostgresConstraintForeignKeyColumnOrder(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	postgresExec(t, s, "CREATE TABLE "+schema+".parent (a integer, b integer, UNIQUE (a, b), UNIQUE (b, a))")
	postgresExec(t, s, "CREATE TABLE "+schema+".child (x integer, y integer, CONSTRAINT fk FOREIGN KEY (x, y) REFERENCES "+schema+".parent (a, b))")

	before := postgresSnapshot(t, s)
	assertForeignKeyColumns(t, before.ForeignKeys[s.schemas[0]+".child.fk"], "fk", s.schemas[0], "child", []string{"x", "y"}, s.schemas[0], "parent", []string{"a", "b"})

	postgresExec(t, s, "ALTER TABLE "+schema+".child DROP CONSTRAINT fk")
	postgresExec(t, s, "ALTER TABLE "+schema+".child ADD CONSTRAINT fk FOREIGN KEY (x, y) REFERENCES "+schema+".parent (b, a)")
	after := postgresSnapshot(t, s)
	assertForeignKeyColumns(t, after.ForeignKeys[s.schemas[0]+".child.fk"], "fk", s.schemas[0], "child", []string{"x", "y"}, s.schemas[0], "parent", []string{"b", "a"})
	assertSnapshotsDiffer(t, before, after)
}

func TestPostgresConstraintAssociationsAndExternalForeignSchema(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	schema1 := pq.QuoteIdentifier(s.schemas[0])
	schema2 := pq.QuoteIdentifier(s.schemas[1])
	externalName := s.schemas[0] + "_external"
	external := pq.QuoteIdentifier(externalName)
	postgresExec(t, s, "CREATE SCHEMA "+external)
	t.Cleanup(func() {
		if _, err := s.dbClient.Execute("DROP SCHEMA " + external + " CASCADE"); err != nil {
			t.Errorf("drop integration schema %q: %v", externalName, err)
		}
	})
	postgresExec(t, s, "CREATE TABLE "+external+".parent (id integer PRIMARY KEY)")
	postgresExec(t, s, "CREATE TABLE "+schema1+".checked (x integer CONSTRAINT shared CHECK (x > 1))")
	postgresExec(t, s, "CREATE TABLE "+schema2+".checked (x integer CONSTRAINT shared CHECK (x < 9))")
	postgresExec(t, s, "CREATE TABLE "+schema1+".child (x integer CONSTRAINT shared REFERENCES "+external+".parent (id))")

	snapshot := postgresSnapshot(t, s)
	assertConstraint(t, snapshot.Constraints[s.schemas[0]+".checked.shared"], s.schemas[0], "checked", "CHECK", "CHECK ((x > 1))")
	assertConstraint(t, snapshot.Constraints[s.schemas[1]+".checked.shared"], s.schemas[1], "checked", "CHECK", "CHECK ((x < 9))")
	assertConstraint(t, snapshot.Constraints[s.schemas[0]+".child.shared"], s.schemas[0], "child", "FOREIGN KEY", "FOREIGN KEY (x) REFERENCES "+externalName+".parent(id)")
	assertForeignKeyColumns(t, snapshot.ForeignKeys[s.schemas[0]+".child.shared"], "shared", s.schemas[0], "child", []string{"x"}, externalName, "parent", []string{"id"})
}

func TestPostgresConstraintDrift(t *testing.T) {
	tests := []struct {
		name   string
		create func(*StaircaseWorker, string)
		alter  func(*StaircaseWorker, string)
		assert func(*testing.T, *driver.SchemaSnapshot, *driver.SchemaSnapshot, string)
	}{
		{
			name: "deferrable initially deferred",
			create: func(s *StaircaseWorker, schema string) {
				createConstraintTables(t, s, schema, "CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id)")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child ALTER CONSTRAINT fk DEFERRABLE INITIALLY DEFERRED")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if before.Constraints[key].Deferrable || before.Constraints[key].InitiallyDeferred || !after.Constraints[key].Deferrable || !after.Constraints[key].InitiallyDeferred {
					t.Fatalf("deferrability did not change as expected: before=%+v after=%+v", before.Constraints[key], after.Constraints[key])
				}
			},
		},
		{
			name: "initially immediate",
			create: func(s *StaircaseWorker, schema string) {
				createConstraintTables(t, s, schema, "CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id) DEFERRABLE INITIALLY DEFERRED")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child ALTER CONSTRAINT fk DEFERRABLE INITIALLY IMMEDIATE")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if !before.Constraints[key].InitiallyDeferred || after.Constraints[key].InitiallyDeferred {
					t.Fatalf("initial mode did not change as expected: before=%+v after=%+v", before.Constraints[key], after.Constraints[key])
				}
			},
		},
		{
			name: "match full",
			create: func(s *StaircaseWorker, schema string) {
				createConstraintTables(t, s, schema, "CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id) MATCH SIMPLE")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child DROP CONSTRAINT fk")
				postgresExec(t, s, "ALTER TABLE "+schema+".child ADD CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id) MATCH FULL")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if strings.Contains(before.ForeignKeys[key].Definition, "MATCH FULL") || !strings.Contains(after.ForeignKeys[key].Definition, "MATCH FULL") {
					t.Fatalf("match definition did not change as expected: before=%q after=%q", before.ForeignKeys[key].Definition, after.ForeignKeys[key].Definition)
				}
			},
		},
		{
			name: "update and delete actions",
			create: func(s *StaircaseWorker, schema string) {
				createConstraintTables(t, s, schema, "CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id)")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child DROP CONSTRAINT fk")
				postgresExec(t, s, "ALTER TABLE "+schema+".child ADD CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id) ON UPDATE CASCADE ON DELETE SET NULL")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if before.ForeignKeys[key].UpdateRule != "NO ACTION" || before.ForeignKeys[key].DeleteRule != "NO ACTION" || after.ForeignKeys[key].UpdateRule != "CASCADE" || after.ForeignKeys[key].DeleteRule != "SET NULL" {
					t.Fatalf("referential actions did not change as expected: before=%+v after=%+v", before.ForeignKeys[key], after.ForeignKeys[key])
				}
			},
		},
		{
			name: "check validation",
			create: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "CREATE TABLE "+schema+".child (x integer)")
				postgresExec(t, s, "ALTER TABLE "+schema+".child ADD CONSTRAINT ck CHECK (x > 0) NOT VALID")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child VALIDATE CONSTRAINT ck")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if before.Constraints[key].Validated || !after.Constraints[key].Validated {
					t.Fatalf("CHECK validation did not change as expected: before=%+v after=%+v", before.Constraints[key], after.Constraints[key])
				}
			},
		},
		{
			name: "check no inherit",
			create: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "CREATE TABLE "+schema+".child (x integer, CONSTRAINT ck CHECK (x > 0))")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child DROP CONSTRAINT ck")
				postgresExec(t, s, "ALTER TABLE "+schema+".child ADD CONSTRAINT ck CHECK (x > 0) NO INHERIT")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if before.Constraints[key].NoInherit || !after.Constraints[key].NoInherit {
					t.Fatalf("CHECK inheritance did not change as expected: before=%+v after=%+v", before.Constraints[key], after.Constraints[key])
				}
			},
		},
		{
			name: "foreign key validation",
			create: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "CREATE TABLE "+schema+".parent (id integer PRIMARY KEY)")
				postgresExec(t, s, "CREATE TABLE "+schema+".child (x integer)")
				postgresExec(t, s, "ALTER TABLE "+schema+".child ADD CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id) NOT VALID")
			},
			alter: func(s *StaircaseWorker, schema string) {
				postgresExec(t, s, "ALTER TABLE "+schema+".child VALIDATE CONSTRAINT fk")
			},
			assert: func(t *testing.T, before, after *driver.SchemaSnapshot, key string) {
				if before.Constraints[key].Validated || !after.Constraints[key].Validated {
					t.Fatalf("foreign key validation did not change as expected: before=%+v after=%+v", before.Constraints[key], after.Constraints[key])
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			schema := pq.QuoteIdentifier(s.schemas[0])
			test.create(s, schema)
			before := postgresSnapshot(t, s)
			test.alter(s, schema)
			after := postgresSnapshot(t, s)
			constraintName := "fk"
			if strings.HasPrefix(test.name, "check ") {
				constraintName = "ck"
			}
			key := s.schemas[0] + ".child." + constraintName
			test.assert(t, before, after, key)
			assertSnapshotsDiffer(t, before, after)
		})
	}
}

func TestPostgresConstraintDefinitionsAndEquivalentRecreation(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	table := schema + ".defined"
	postgresExec(t, s, "CREATE TABLE "+table+" (id integer, email text, score integer, span int4range, CONSTRAINT pk PRIMARY KEY (id), CONSTRAINT uq UNIQUE (email), CONSTRAINT ck CHECK (score > 0), CONSTRAINT ex EXCLUDE USING gist (span WITH &&))")

	before := postgresSnapshot(t, s)
	assertConstraint(t, before.Constraints[s.schemas[0]+".defined.ck"], s.schemas[0], "defined", "CHECK", "CHECK ((score > 0))")
	assertConstraint(t, before.Constraints[s.schemas[0]+".defined.uq"], s.schemas[0], "defined", "UNIQUE", "UNIQUE (email)")
	assertConstraint(t, before.Constraints[s.schemas[0]+".defined.pk"], s.schemas[0], "defined", "PRIMARY KEY", "PRIMARY KEY (id)")
	assertConstraint(t, before.Constraints[s.schemas[0]+".defined.ex"], s.schemas[0], "defined", "EXCLUDE", "EXCLUDE USING gist (span WITH &&)")

	postgresExec(t, s, "ALTER TABLE "+table+" DROP CONSTRAINT ck, DROP CONSTRAINT uq, DROP CONSTRAINT pk, DROP CONSTRAINT ex")
	postgresExec(t, s, "ALTER TABLE "+table+" ADD CONSTRAINT pk PRIMARY KEY (id), ADD CONSTRAINT uq UNIQUE (email), ADD CONSTRAINT ck CHECK (score > 0), ADD CONSTRAINT ex EXCLUDE USING gist (span WITH &&)")
	after := postgresSnapshot(t, s)
	if err := compareSchemas(before, after); err != nil {
		t.Fatalf("equivalent named constraint recreation changed snapshot: %v", err)
	}
}

func TestPostgresConstraintForeignKeyEnforcement(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	version := postgresServerVersion(t, s)
	if version < 180000 {
		t.Skipf("PostgreSQL %d does not support foreign key enforcement changes", version)
	}
	schema := pq.QuoteIdentifier(s.schemas[0])
	createConstraintTables(t, s, schema, "CONSTRAINT fk FOREIGN KEY (x) REFERENCES "+schema+".parent (id)")
	key := s.schemas[0] + ".child.fk"

	before := postgresSnapshot(t, s)
	if !before.Constraints[key].Enforced {
		t.Fatalf("initial foreign key is not enforced: %+v", before.Constraints[key])
	}
	postgresExec(t, s, "ALTER TABLE "+schema+".child ALTER CONSTRAINT fk NOT ENFORCED")
	after := postgresSnapshot(t, s)
	if after.Constraints[key].Enforced {
		t.Fatalf("NOT ENFORCED foreign key is mapped as enforced: %+v", after.Constraints[key])
	}
	if after.Constraints[key].Validated {
		t.Fatalf("NOT ENFORCED foreign key is mapped as validated: %+v", after.Constraints[key])
	}
	assertSnapshotsDiffer(t, before, after)

	postgresExec(t, s, "ALTER TABLE "+schema+".child ALTER CONSTRAINT fk ENFORCED")
	postgresExec(t, s, "ALTER TABLE "+schema+".child VALIDATE CONSTRAINT fk")
	if err := compareSchemas(before, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("restoring foreign key enforcement changed snapshot: %v", err)
	}
}

func createConstraintTables(t *testing.T, s *StaircaseWorker, schema, constraint string) {
	t.Helper()
	postgresExec(t, s, "CREATE TABLE "+schema+".parent (id integer PRIMARY KEY)")
	postgresExec(t, s, "CREATE TABLE "+schema+".child (x integer, "+constraint+")")
}

func assertForeignKeyColumns(t *testing.T, got driver.ForeignKeyDefinition, constraintName, tableSchema, tableName string, columns []string, foreignTableSchema, foreignTableName string, foreignColumns []string) {
	t.Helper()
	if got.ConstraintName != constraintName || got.TableSchema != tableSchema || got.TableName != tableName || !reflect.DeepEqual(got.ColumnNames, columns) || got.ForeignTableSchema != foreignTableSchema || got.ForeignTableName != foreignTableName || !reflect.DeepEqual(got.ForeignColumnNames, foreignColumns) {
		t.Fatalf("foreign key = %+v, want name %q table %q.%q columns %v foreign table %q.%q columns %v", got, constraintName, tableSchema, tableName, columns, foreignTableSchema, foreignTableName, foreignColumns)
	}
}

func assertConstraint(t *testing.T, got driver.ConstraintDefinition, tableSchema, tableName, constraintType, definition string) {
	t.Helper()
	if got.TableSchema != tableSchema || got.TableName != tableName || got.ConstraintType != constraintType || !got.Definition.Valid || got.Definition.String != definition {
		t.Fatalf("constraint = %+v, want schema=%q table=%q type=%q definition=%q", got, tableSchema, tableName, constraintType, definition)
	}
}

func assertSnapshotsDiffer(t *testing.T, before, after *driver.SchemaSnapshot) {
	t.Helper()
	if err := compareSchemas(before, after); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("schema comparison error = %v, want ErrSnapshotsDiffer", err)
	}
}

func postgresServerVersion(t *testing.T, s *StaircaseWorker) int {
	t.Helper()
	result, err := s.dbClient.Execute("SELECT current_setting('server_version_num')::integer")
	if err != nil {
		t.Fatalf("query PostgreSQL server version: %v", err)
	}
	defer result.Rows.Close()
	if !result.Rows.Next() {
		t.Fatal("PostgreSQL server version query returned no rows")
	}
	var version int
	if err := result.Rows.Scan(&version); err != nil {
		t.Fatalf("scan PostgreSQL server version: %v", err)
	}
	if err := result.Rows.Err(); err != nil {
		t.Fatalf("iterate PostgreSQL server version rows: %v", err)
	}
	return version
}
