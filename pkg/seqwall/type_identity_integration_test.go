package seqwall

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

func TestPostgresTypeRecreation(t *testing.T) {
	tests := []struct {
		name         string
		createTypes  func(string) []string
		dropTypes    func(string) []string
		columnType   func(string) string
		wantIdentity func(string) string
	}{
		{
			name: "scalar enum",
			createTypes: func(schema string) []string {
				return []string{"CREATE TYPE " + schema + ".mood AS ENUM ('ok','bad')"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP TYPE " + schema + ".mood"} },
			columnType:   func(schema string) string { return schema + ".mood" },
			wantIdentity: func(schema string) string { return schema + ".mood" },
		},
		{
			name: "enum array",
			createTypes: func(schema string) []string {
				return []string{"CREATE TYPE " + schema + ".mood AS ENUM ('ok','bad')"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP TYPE " + schema + ".mood"} },
			columnType:   func(schema string) string { return schema + ".mood[]" },
			wantIdentity: func(schema string) string { return schema + ".mood[]" },
		},
		{
			name: "domain over integer",
			createTypes: func(schema string) []string {
				return []string{"CREATE DOMAIN " + schema + ".amount AS integer"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP DOMAIN " + schema + ".amount"} },
			columnType:   func(schema string) string { return schema + ".amount" },
			wantIdentity: func(schema string) string { return schema + ".amount" },
		},
		{
			name: "nested domains",
			createTypes: func(schema string) []string {
				return []string{
					"CREATE DOMAIN " + schema + ".amount AS integer",
					"CREATE DOMAIN " + schema + ".nested_amount AS " + schema + ".amount",
				}
			},
			dropTypes: func(schema string) []string {
				return []string{
					"DROP DOMAIN " + schema + ".nested_amount",
					"DROP DOMAIN " + schema + ".amount",
				}
			},
			columnType:   func(schema string) string { return schema + ".nested_amount" },
			wantIdentity: func(schema string) string { return schema + ".nested_amount" },
		},
		{
			name: "domain over enum",
			createTypes: func(schema string) []string {
				return []string{
					"CREATE TYPE " + schema + ".mood AS ENUM ('ok','bad')",
					"CREATE DOMAIN " + schema + ".mood_domain AS " + schema + ".mood",
				}
			},
			dropTypes: func(schema string) []string {
				return []string{
					"DROP DOMAIN " + schema + ".mood_domain",
					"DROP TYPE " + schema + ".mood",
				}
			},
			columnType:   func(schema string) string { return schema + ".mood_domain" },
			wantIdentity: func(schema string) string { return schema + ".mood_domain" },
		},
		{
			name: "domain over array",
			createTypes: func(schema string) []string {
				return []string{"CREATE DOMAIN " + schema + ".amounts AS integer[]"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP DOMAIN " + schema + ".amounts"} },
			columnType:   func(schema string) string { return schema + ".amounts" },
			wantIdentity: func(schema string) string { return schema + ".amounts" },
		},
		{
			name: "array of domain",
			createTypes: func(schema string) []string {
				return []string{"CREATE DOMAIN " + schema + ".amount AS integer"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP DOMAIN " + schema + ".amount"} },
			columnType:   func(schema string) string { return schema + ".amount[]" },
			wantIdentity: func(schema string) string { return schema + ".amount[]" },
		},
		{
			name: "standalone composite",
			createTypes: func(schema string) []string {
				return []string{"CREATE TYPE " + schema + ".pair AS (x integer,y text)"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP TYPE " + schema + ".pair"} },
			columnType:   func(schema string) string { return schema + ".pair" },
			wantIdentity: func(schema string) string { return schema + ".pair" },
		},
		{
			name: "composite array",
			createTypes: func(schema string) []string {
				return []string{"CREATE TYPE " + schema + ".pair AS (x integer,y text)"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP TYPE " + schema + ".pair"} },
			columnType:   func(schema string) string { return schema + ".pair[]" },
			wantIdentity: func(schema string) string { return schema + ".pair[]" },
		},
		{
			name: "custom range",
			createTypes: func(schema string) []string {
				return []string{"CREATE TYPE " + schema + ".span AS RANGE (subtype=integer)"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP TYPE " + schema + ".span"} },
			columnType:   func(schema string) string { return schema + ".span" },
			wantIdentity: func(schema string) string { return schema + ".span" },
		},
		{
			name: "range array",
			createTypes: func(schema string) []string {
				return []string{"CREATE TYPE " + schema + ".span AS RANGE (subtype=integer)"}
			},
			dropTypes:    func(schema string) []string { return []string{"DROP TYPE " + schema + ".span"} },
			columnType:   func(schema string) string { return schema + ".span[]" },
			wantIdentity: func(schema string) string { return schema + ".span[]" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			schemaName := s.schemas[0]
			schema := pq.QuoteIdentifier(schemaName)
			create := func() {
				for _, query := range tt.createTypes(schema) {
					postgresExec(t, s, query)
				}
				postgresExec(t, s, "CREATE TABLE "+schema+".items (value "+tt.columnType(schema)+")")
			}

			create()
			before := postgresSnapshot(t, s)
			beforeOID := postgresItemValueTypeOID(t, s, schemaName)
			postgresExec(t, s, "DROP TABLE "+schema+".items")
			for _, query := range tt.dropTypes(schema) {
				postgresExec(t, s, query)
			}
			create()
			after := postgresSnapshot(t, s)
			afterOID := postgresItemValueTypeOID(t, s, schemaName)

			if beforeOID == afterOID {
				t.Fatalf("declared type OID remained %d after full recreation", beforeOID)
			}
			if err := compareSchemas(before, after); err != nil {
				t.Fatalf("identical type recreation changed snapshot: %v", err)
			}
			assertColumnTypeIdentity(t, before, schemaName+".items", "value", tt.wantIdentity(schemaName))
			assertColumnTypeIdentity(t, after, schemaName+".items", "value", tt.wantIdentity(schemaName))
		})
	}
}

func TestPostgresTypeArrayNameCollisionRecreation(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schemaName := s.schemas[0]
	schema := pq.QuoteIdentifier(schemaName)
	createTable := func() {
		postgresExec(t, s, "CREATE TABLE "+schema+".items (value "+schema+".mood[])")
	}

	postgresExec(t, s, "CREATE TYPE "+schema+".mood AS ENUM ('ok','bad')")
	postgresExec(t, s, "CREATE TYPE "+schema+"._mood AS (x integer,y text)")
	createTable()
	before := postgresSnapshot(t, s)
	beforeOID := postgresItemValueTypeOID(t, s, schemaName)
	beforeComposite := postgresCompositeAttributes(t, s, schemaName, "_mood")

	postgresExec(t, s, "DROP TABLE "+schema+".items")
	postgresExec(t, s, "DROP TYPE "+schema+"._mood")
	postgresExec(t, s, "DROP TYPE "+schema+".mood")
	postgresExec(t, s, "CREATE TYPE "+schema+"._mood AS (x integer,y text)")
	postgresExec(t, s, "CREATE TYPE "+schema+".mood AS ENUM ('ok','bad')")
	createTable()
	after := postgresSnapshot(t, s)
	afterOID := postgresItemValueTypeOID(t, s, schemaName)
	afterComposite := postgresCompositeAttributes(t, s, schemaName, "_mood")

	if beforeOID == afterOID {
		t.Fatalf("declared array type OID remained %d after full recreation", beforeOID)
	}
	beforeColumn := snapshotColumn(t, before, schemaName+".items", "value")
	afterColumn := snapshotColumn(t, after, schemaName+".items", "value")
	if beforeColumn.UDTName == afterColumn.UDTName {
		t.Fatalf("generated array UDT name remained %q across reversed type creation order", beforeColumn.UDTName)
	}
	if got, want := beforeColumn.TypeMeta.TypeIdentity, schemaName+".mood[]"; got != want {
		t.Fatalf("before type identity = %q, want %q", got, want)
	}
	if got, want := afterColumn.TypeMeta.TypeIdentity, schemaName+".mood[]"; got != want {
		t.Fatalf("after type identity = %q, want %q", got, want)
	}
	wantComposite := []string{"x:integer", "y:text"}
	if !reflect.DeepEqual(beforeComposite, wantComposite) || !reflect.DeepEqual(afterComposite, wantComposite) {
		t.Fatalf("composite attributes before/after = %v/%v, want %v", beforeComposite, afterComposite, wantComposite)
	}
	wantLabels := []string{"ok", "bad"}
	if !reflect.DeepEqual(before.EnumTypes[schemaName+".mood"].Labels, wantLabels) || !reflect.DeepEqual(after.EnumTypes[schemaName+".mood"].Labels, wantLabels) {
		t.Fatalf("enum labels before/after = %v/%v, want %v", before.EnumTypes[schemaName+".mood"].Labels, after.EnumTypes[schemaName+".mood"].Labels, wantLabels)
	}
	if err := compareSchemas(before, after); err != nil {
		t.Fatalf("identical user-visible type recreation changed snapshot: %v", err)
	}
}

func TestPostgresTypeIdentityDrift(t *testing.T) {
	tests := []struct {
		name        string
		createTypes func(string, string) []string
		sourceType  func(string, string) string
		targetType  func(string, string) string
		using       func(string, string) string
		wantBefore  func(string, string) string
		wantAfter   func(string, string) string
	}{
		{
			name: "same-name enum across schemas",
			createTypes: func(first, second string) []string {
				return []string{
					"CREATE TYPE " + first + ".mood AS ENUM ('ok','bad')",
					"CREATE TYPE " + second + ".mood AS ENUM ('ok','bad')",
				}
			},
			sourceType: func(first, _ string) string { return first + ".mood" },
			targetType: func(_, second string) string { return second + ".mood" },
			using:      func(_, second string) string { return "value::text::" + second + ".mood" },
			wantBefore: func(first, _ string) string { return first + ".mood" },
			wantAfter:  func(_, second string) string { return second + ".mood" },
		},
		{
			name: "domain to base",
			createTypes: func(first, _ string) []string {
				return []string{"CREATE DOMAIN " + first + ".amount AS integer"}
			},
			sourceType: func(first, _ string) string { return first + ".amount" },
			targetType: func(_, _ string) string { return "integer" },
			using:      func(_, _ string) string { return "NULL::integer" },
			wantBefore: func(first, _ string) string { return first + ".amount" },
			wantAfter:  func(_, _ string) string { return "pg_catalog.int4" },
		},
		{
			name: "base to domain",
			createTypes: func(first, _ string) []string {
				return []string{"CREATE DOMAIN " + first + ".amount AS integer"}
			},
			sourceType: func(_, _ string) string { return "integer" },
			targetType: func(first, _ string) string { return first + ".amount" },
			using:      func(first, _ string) string { return "NULL::" + first + ".amount" },
			wantBefore: func(_, _ string) string { return "pg_catalog.int4" },
			wantAfter:  func(first, _ string) string { return first + ".amount" },
		},
		{
			name: "same-name domain across schemas",
			createTypes: func(first, second string) []string {
				return []string{
					"CREATE DOMAIN " + first + ".amount AS integer",
					"CREATE DOMAIN " + second + ".amount AS integer",
				}
			},
			sourceType: func(first, _ string) string { return first + ".amount" },
			targetType: func(_, second string) string { return second + ".amount" },
			using:      func(_, second string) string { return "NULL::" + second + ".amount" },
			wantBefore: func(first, _ string) string { return first + ".amount" },
			wantAfter:  func(_, second string) string { return second + ".amount" },
		},
		{
			name: "same-name composite across schemas",
			createTypes: func(first, second string) []string {
				return []string{
					"CREATE TYPE " + first + ".pair AS (x integer,y text)",
					"CREATE TYPE " + second + ".pair AS (x integer,y text)",
				}
			},
			sourceType: func(first, _ string) string { return first + ".pair" },
			targetType: func(_, second string) string { return second + ".pair" },
			using:      func(_, second string) string { return "NULL::" + second + ".pair" },
			wantBefore: func(first, _ string) string { return first + ".pair" },
			wantAfter:  func(_, second string) string { return second + ".pair" },
		},
		{
			name: "same-name range across schemas",
			createTypes: func(first, second string) []string {
				return []string{
					"CREATE TYPE " + first + ".span AS RANGE (subtype=integer)",
					"CREATE TYPE " + second + ".span AS RANGE (subtype=integer)",
				}
			},
			sourceType: func(first, _ string) string { return first + ".span" },
			targetType: func(_, second string) string { return second + ".span" },
			using:      func(_, second string) string { return "NULL::" + second + ".span" },
			wantBefore: func(first, _ string) string { return first + ".span" },
			wantAfter:  func(_, second string) string { return second + ".span" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 2)
			firstName, secondName := s.schemas[0], s.schemas[1]
			first, second := pq.QuoteIdentifier(firstName), pq.QuoteIdentifier(secondName)
			for _, query := range tt.createTypes(first, second) {
				postgresExec(t, s, query)
			}
			postgresExec(t, s, "CREATE TABLE "+first+".items (value "+tt.sourceType(first, second)+")")

			before := postgresSnapshot(t, s)
			assertColumnTypeIdentity(t, before, firstName+".items", "value", tt.wantBefore(firstName, secondName))
			postgresExec(t, s, "ALTER TABLE "+first+".items ALTER COLUMN value TYPE "+tt.targetType(first, second)+" USING "+tt.using(first, second))
			after := postgresSnapshot(t, s)
			assertColumnTypeIdentity(t, after, firstName+".items", "value", tt.wantAfter(firstName, secondName))
			if err := compareSchemas(before, after); !errors.Is(err, ErrSnapshotsDiffer()) {
				t.Fatalf("logical type change comparison error = %v, want ErrSnapshotsDiffer", err)
			}
		})
	}
}

func TestPostgresTypeModifiers(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schemaName := s.schemas[0]
	schema := pq.QuoteIdentifier(schemaName)
	postgresExec(t, s, "CREATE TABLE "+schema+".items (words varchar(10)[], duration interval DAY)")

	before := postgresSnapshot(t, s)
	assertColumnType(t, before, schemaName+".items", "words", `pg_catalog."varchar"[]`, 14)
	beforeInterval := snapshotColumn(t, before, schemaName+".items", "duration")
	assertColumnTypeIdentity(t, before, schemaName+".items", "duration", `pg_catalog."interval"`)
	postgresExec(t, s, "ALTER TABLE "+schema+".items ALTER COLUMN words TYPE varchar(20)[] USING words::varchar(20)[]")
	postgresExec(t, s, "ALTER TABLE "+schema+".items ALTER COLUMN duration TYPE interval HOUR USING duration::interval HOUR")
	after := postgresSnapshot(t, s)
	assertColumnType(t, after, schemaName+".items", "words", `pg_catalog."varchar"[]`, 24)
	afterInterval := snapshotColumn(t, after, schemaName+".items", "duration")
	assertColumnTypeIdentity(t, after, schemaName+".items", "duration", `pg_catalog."interval"`)
	if beforeInterval.TypeMeta.TypeModifier == afterInterval.TypeMeta.TypeModifier {
		t.Fatalf("interval type modifier remained %d after DAY to HOUR change", beforeInterval.TypeMeta.TypeModifier)
	}
	if err := compareSchemas(before, after); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("type modifier comparison error = %v, want ErrSnapshotsDiffer", err)
	}
}

func TestPostgresTypeCollationNamespace(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	firstName, secondName := s.schemas[0], s.schemas[1]
	first, second := pq.QuoteIdentifier(firstName), pq.QuoteIdentifier(secondName)
	postgresExec(t, s, "CREATE COLLATION "+first+".same_collation FROM pg_catalog.\"C\"")
	postgresExec(t, s, "CREATE COLLATION "+second+".same_collation FROM pg_catalog.\"C\"")
	postgresExec(t, s, "CREATE TABLE "+first+".items (value text COLLATE "+first+".same_collation)")

	before := postgresSnapshot(t, s)
	assertColumnCollation(t, before, firstName+".items", "value", firstName, "same_collation")
	postgresExec(t, s, "ALTER TABLE "+first+".items ALTER COLUMN value TYPE text COLLATE "+second+".same_collation")
	after := postgresSnapshot(t, s)
	assertColumnCollation(t, after, firstName+".items", "value", secondName, "same_collation")
	if err := compareSchemas(before, after); !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Fatalf("collation namespace comparison error = %v, want ErrSnapshotsDiffer", err)
	}
}

func postgresItemValueTypeOID(t *testing.T, s *StaircaseWorker, schema string) int {
	t.Helper()
	result, err := s.dbClient.Execute(`
		SELECT a.atttypid
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class r ON r.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = r.relnamespace
		WHERE n.nspname = $1 AND r.relname = $2 AND a.attname = $3
	`, schema, "items", "value")
	if err != nil {
		t.Fatalf("query column type OID: %v", err)
	}
	defer result.Rows.Close()
	if !result.Rows.Next() {
		t.Fatalf("column %s.items.value has no type OID", schema)
	}
	var oid int
	if err := result.Rows.Scan(&oid); err != nil {
		t.Fatalf("scan column type OID: %v", err)
	}
	return oid
}

func postgresCompositeAttributes(t *testing.T, s *StaircaseWorker, schema, typeName string) []string {
	t.Helper()
	result, err := s.dbClient.Execute(`
		SELECT a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod)
		FROM pg_catalog.pg_type t
		JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		JOIN pg_catalog.pg_class r ON r.oid = t.typrelid
		JOIN pg_catalog.pg_attribute a ON a.attrelid = r.oid
		WHERE n.nspname = $1 AND t.typname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum
	`, schema, typeName)
	if err != nil {
		t.Fatalf("query composite attributes: %v", err)
	}
	defer result.Rows.Close()
	var attributes []string
	for result.Rows.Next() {
		var name, dataType string
		if err := result.Rows.Scan(&name, &dataType); err != nil {
			t.Fatalf("scan composite attribute: %v", err)
		}
		attributes = append(attributes, name+":"+dataType)
	}
	if err := result.Rows.Err(); err != nil {
		t.Fatalf("iterate composite attributes: %v", err)
	}
	return attributes
}

func snapshotColumn(t *testing.T, snapshot *driver.SchemaSnapshot, tableName, columnName string) driver.ColumnDefinition {
	t.Helper()
	table, ok := snapshot.Tables[tableName]
	if !ok {
		t.Fatalf("table %q missing from snapshot", tableName)
	}
	for _, column := range table.Columns {
		if column.ColumnName == columnName {
			return column
		}
	}
	t.Fatalf("column %q missing from table %q", columnName, tableName)
	return driver.ColumnDefinition{}
}

func assertColumnTypeIdentity(t *testing.T, snapshot *driver.SchemaSnapshot, tableName, columnName, want string) {
	t.Helper()
	column := snapshotColumn(t, snapshot, tableName, columnName)
	if got := column.TypeMeta.TypeIdentity; got != want {
		t.Fatalf("%s.%s type identity = %q, want %q", tableName, columnName, got, want)
	}
	if column.TypeMeta.TypeOID == 0 {
		t.Fatalf("%s.%s diagnostic type OID = 0, want catalog OID", tableName, columnName)
	}
	encoded, err := json.Marshal(column.TypeMeta)
	if err != nil {
		t.Fatalf("marshal type metadata: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("unmarshal type metadata: %v", err)
	}
	if _, ok := fields["type_oid"]; ok {
		t.Fatalf("serialized type metadata contains diagnostic OID: %s", encoded)
	}
}

func assertColumnType(t *testing.T, snapshot *driver.SchemaSnapshot, tableName, columnName, wantIdentity string, wantModifier int) {
	t.Helper()
	column := snapshotColumn(t, snapshot, tableName, columnName)
	assertColumnTypeIdentity(t, snapshot, tableName, columnName, wantIdentity)
	if got := column.TypeMeta.TypeModifier; got != wantModifier {
		t.Fatalf("%s.%s type modifier = %d, want %d", tableName, columnName, got, wantModifier)
	}
}

func assertColumnCollation(t *testing.T, snapshot *driver.SchemaSnapshot, tableName, columnName, wantSchema, wantName string) {
	t.Helper()
	column := snapshotColumn(t, snapshot, tableName, columnName)
	if !column.CollationSchema.Valid || column.CollationSchema.String != wantSchema {
		t.Fatalf("%s.%s collation schema = %+v, want %q", tableName, columnName, column.CollationSchema, wantSchema)
	}
	if !column.CollationName.Valid || column.CollationName.String != wantName {
		t.Fatalf("%s.%s collation name = %+v, want %q", tableName, columnName, column.CollationName, wantName)
	}
}
