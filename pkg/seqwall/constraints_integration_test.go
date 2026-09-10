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

func TestPostgresNotNullRecreation(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	table := schema + ".not_null_recreation"
	columnNames := []string{"имя", "with space", "price$", "a.b", `embedded"quote`}
	definitions := make([]string, 0, len(columnNames))
	for _, columnName := range columnNames {
		definitions = append(definitions, pq.QuoteIdentifier(columnName)+" integer NOT NULL")
	}
	create := "CREATE TABLE " + table + " (" + strings.Join(definitions, ", ") + ")"

	postgresExec(t, s, create)
	before := postgresSnapshot(t, s)
	for _, columnName := range columnNames {
		assertColumnNullable(t, before, s.schemas[0]+".not_null_recreation", columnName, "NO")
	}
	postgresExec(t, s, "DROP TABLE "+table)
	postgresExec(t, s, create)
	after := postgresSnapshot(t, s)
	if err := compareSchemas(before, after); err != nil {
		t.Fatalf("identical NOT NULL recreation changed snapshot: %v", err)
	}

	postgresExec(t, s, "CREATE TABLE "+schema+".a_b (c integer NOT NULL)")
	postgresExec(t, s, "CREATE TABLE "+schema+".a (b_c integer NOT NULL)")
	postgresExec(t, s, "CREATE TABLE "+schema+".named_checks (c integer CONSTRAINT native_nn NOT NULL, CONSTRAINT a_b_c_not_null CHECK (c IS NOT NULL), CONSTRAINT explicit_check CHECK (c IS NOT NULL))")
	baseline := postgresSnapshot(t, s)
	assertColumnNullable(t, baseline, s.schemas[0]+".a_b", "c", "NO")
	assertColumnNullable(t, baseline, s.schemas[0]+".a", "b_c", "NO")
	assertConstraint(t, baseline.Constraints[s.schemas[0]+".named_checks.a_b_c_not_null"], s.schemas[0], "named_checks", "CHECK", "CHECK ((c IS NOT NULL))")
	assertConstraint(t, baseline.Constraints[s.schemas[0]+".named_checks.explicit_check"], s.schemas[0], "named_checks", "CHECK", "CHECK ((c IS NOT NULL))")

	version := postgresServerVersion(t, s)
	if version >= 180000 {
		assertConstraint(t, baseline.Constraints[s.schemas[0]+".a_b.a_b_c_not_null"], s.schemas[0], "a_b", "NOT NULL", "NOT NULL c")
		assertConstraint(t, baseline.Constraints[s.schemas[0]+".a.a_b_c_not_null1"], s.schemas[0], "a", "NOT NULL", "NOT NULL b_c")
		assertConstraint(t, baseline.Constraints[s.schemas[0]+".named_checks.native_nn"], s.schemas[0], "named_checks", "NOT NULL", "NOT NULL c")
	}

	postgresExec(t, s, "ALTER TABLE "+schema+".a_b ALTER COLUMN c DROP NOT NULL")
	nullabilityChanged := postgresSnapshot(t, s)
	assertColumnNullable(t, nullabilityChanged, s.schemas[0]+".a_b", "c", "YES")
	assertColumnNullable(t, nullabilityChanged, s.schemas[0]+".a", "b_c", "NO")
	assertSnapshotsDiffer(t, baseline, nullabilityChanged)

	postgresExec(t, s, "ALTER TABLE "+schema+".named_checks RENAME CONSTRAINT a_b_c_not_null TO renamed_check")
	renamed := postgresSnapshot(t, s)
	assertConstraint(t, renamed.Constraints[s.schemas[0]+".named_checks.renamed_check"], s.schemas[0], "named_checks", "CHECK", "CHECK ((c IS NOT NULL))")
	assertSnapshotsDiffer(t, nullabilityChanged, renamed)
	postgresExec(t, s, "ALTER TABLE "+schema+".named_checks RENAME CONSTRAINT renamed_check TO a_b_c_not_null")
	if err := compareSchemas(nullabilityChanged, postgresSnapshot(t, s)); err != nil {
		t.Fatalf("restoring explicit CHECK name changed snapshot: %v", err)
	}

	postgresExec(t, s, "ALTER TABLE "+schema+".named_checks DROP CONSTRAINT explicit_check")
	dropped := postgresSnapshot(t, s)
	assertColumnNullable(t, dropped, s.schemas[0]+".named_checks", "c", "NO")
	assertSnapshotsDiffer(t, nullabilityChanged, dropped)
}

func TestPostgresNotNullConstraintMetadata(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	version := postgresServerVersion(t, s)
	if version < 180000 {
		t.Skipf("PostgreSQL %d does not expose NOT NULL constraints", version)
	}
	schema := pq.QuoteIdentifier(s.schemas[0])
	table := schema + ".items"
	postgresExec(t, s, "CREATE TABLE "+table+" (x integer)")
	postgresExec(t, s, "ALTER TABLE "+table+" ADD CONSTRAINT nn NOT NULL x NOT VALID")
	postgresExec(t, s, "ALTER TABLE "+table+" ADD CONSTRAINT check_nn CHECK (x IS NOT NULL)")
	key := s.schemas[0] + ".items.nn"
	checkKey := s.schemas[0] + ".items.check_nn"

	before := postgresSnapshot(t, s)
	assertConstraint(t, before.Constraints[key], s.schemas[0], "items", "NOT NULL", "NOT NULL x NOT VALID")
	assertConstraint(t, before.Constraints[checkKey], s.schemas[0], "items", "CHECK", "CHECK ((x IS NOT NULL))")
	if before.Constraints[key].Validated || before.Constraints[key].NoInherit || !before.Constraints[key].Enforced {
		t.Fatalf("initial NOT NULL metadata = %+v, want enforced, not validated, and inheritable", before.Constraints[key])
	}

	postgresExec(t, s, "ALTER TABLE "+table+" VALIDATE CONSTRAINT nn")
	validated := postgresSnapshot(t, s)
	if !validated.Constraints[key].Validated {
		t.Fatalf("validated NOT NULL metadata = %+v, want validated", validated.Constraints[key])
	}
	assertSnapshotsDiffer(t, before, validated)

	postgresExec(t, s, "ALTER TABLE "+table+" ALTER CONSTRAINT nn NO INHERIT")
	noInherit := postgresSnapshot(t, s)
	if !noInherit.Constraints[key].NoInherit {
		t.Fatalf("NO INHERIT NOT NULL metadata = %+v, want no_inherit", noInherit.Constraints[key])
	}
	assertSnapshotsDiffer(t, validated, noInherit)

	postgresExec(t, s, "ALTER TABLE "+table+" RENAME CONSTRAINT nn TO renamed_nn")
	renamed := postgresSnapshot(t, s)
	if _, ok := renamed.Constraints[key]; ok {
		t.Fatalf("old NOT NULL constraint key %q remains after rename", key)
	}
	assertConstraint(t, renamed.Constraints[s.schemas[0]+".items.renamed_nn"], s.schemas[0], "items", "NOT NULL", "NOT NULL x NO INHERIT")
	assertConstraint(t, renamed.Constraints[checkKey], s.schemas[0], "items", "CHECK", "CHECK ((x IS NOT NULL))")
	assertSnapshotsDiffer(t, noInherit, renamed)
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

func assertColumnNullable(t *testing.T, snapshot *driver.SchemaSnapshot, tableKey, columnName, want string) {
	t.Helper()
	table, ok := snapshot.Tables[tableKey]
	if !ok {
		t.Fatalf("table %q missing from snapshot", tableKey)
	}
	for _, column := range table.Columns {
		if column.ColumnName == columnName {
			if column.IsNullable != want {
				t.Fatalf("column %q.%q nullability = %q, want %q", tableKey, columnName, column.IsNullable, want)
			}
			return
		}
	}
	t.Fatalf("column %q.%q missing from snapshot", tableKey, columnName)
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
