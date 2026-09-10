package seqwall

import (
	"strings"
	"testing"

	"github.com/lib/pq"
)

func TestPostgresSnapshotQuotedTableAndColumnIdentifiers(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	tableName := "Order.Items Details"
	columnName := "Product.ID"
	postgresExec(t, s, "CREATE TABLE "+schema+"."+pq.QuoteIdentifier(tableName)+" ("+pq.QuoteIdentifier(columnName)+" integer NOT NULL)")

	snapshot := postgresSnapshot(t, s)
	table, ok := snapshot.Tables[s.schemas[0]+`."Order.Items Details"`]
	if !ok {
		t.Fatalf("quoted table %q missing from snapshot", tableName)
	}
	if len(table.Columns) != 1 {
		t.Fatalf("quoted table %q has %d columns, want 1", tableName, len(table.Columns))
	}
	if got := table.Columns[0].ColumnName; got != columnName {
		t.Fatalf("quoted column name = %q, want %q", got, columnName)
	}
}

func TestPostgresSnapshotDoesNotDuplicateColumnsForSameNamedTypes(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 2)
	selectedSchema := pq.QuoteIdentifier(s.schemas[0])
	otherSchema := pq.QuoteIdentifier(s.schemas[1])
	s.schemas = s.schemas[:1]
	typeName := pq.QuoteIdentifier("status.kind")
	postgresExec(t, s, "CREATE TYPE "+selectedSchema+"."+typeName+" AS ENUM ('selected')")
	postgresExec(t, s, "CREATE TYPE "+otherSchema+"."+typeName+" AS ENUM ('other')")
	postgresExec(t, s, "CREATE TABLE "+selectedSchema+".typed_rows (state "+selectedSchema+"."+typeName+")")

	snapshot := postgresSnapshot(t, s)
	columns := snapshot.Tables[s.schemas[0]+".typed_rows"].Columns
	if len(columns) != 1 {
		t.Fatalf("typed_rows has %d columns, want 1: %+v", len(columns), columns)
	}
}

func TestPostgresSnapshotEnumDomainRecreation(t *testing.T) {
	for _, nested := range []bool{false, true} {
		name := "direct"
		if nested {
			name = "nested"
		}
		t.Run(name, func(t *testing.T) {
			s := newPostgresIntegrationWorker(t, 1)
			schema := pq.QuoteIdentifier(s.schemas[0])
			postgresExec(t, s, "CREATE TYPE "+schema+".status AS ENUM ('pending', 'done')")
			postgresExec(t, s, "CREATE DOMAIN "+schema+".amount_domain AS integer")
			create := func() {
				postgresExec(t, s, "CREATE DOMAIN "+schema+".status_domain AS "+schema+".status")
				columnType := schema + ".status_domain"
				if nested {
					postgresExec(t, s, "CREATE DOMAIN "+schema+".nested_status_domain AS "+columnType)
					columnType = schema + ".nested_status_domain"
				}
				postgresExec(t, s, "CREATE TABLE "+schema+".items (state "+columnType+", amount "+schema+".amount_domain)")
			}
			create()
			before := postgresSnapshot(t, s)
			postgresExec(t, s, "DROP TABLE "+schema+".items")
			if nested {
				postgresExec(t, s, "DROP DOMAIN "+schema+".nested_status_domain")
			}
			postgresExec(t, s, "DROP DOMAIN "+schema+".status_domain")
			create()
			after := postgresSnapshot(t, s)
			if err := compareSchemas(before, after); err != nil {
				t.Fatalf("identical enum-domain recreation changed snapshot: %v", err)
			}
			columns := after.Tables[s.schemas[0]+".items"].Columns
			if len(columns) != 2 {
				t.Fatalf("items has %d columns, want 2: %+v", len(columns), columns)
			}
			state := columns[0]
			if state.TypeMeta.Typtype != "d" || state.TypeMeta.Typcategory != "E" || state.TypeMeta.TypeOID != 0 {
				t.Fatalf("enum-domain metadata = %+v, want domain/enum category with stable OID", state.TypeMeta)
			}
			amount := columns[1]
			if amount.TypeMeta.Typtype != "d" || amount.TypeMeta.Typcategory != "N" || amount.TypeMeta.TypeOID == 0 {
				t.Fatalf("integer-domain metadata = %+v, want domain/numeric category with catalog OID", amount.TypeMeta)
			}
			postgresExec(t, s, "ALTER TABLE "+schema+".items ALTER COLUMN state TYPE "+schema+".status USING state::"+schema+".status")
			if err := compareSchemas(after, postgresSnapshot(t, s)); err == nil {
				t.Fatal("changing an enum-domain column to its base enum did not change the snapshot")
			}
		})
	}
}

func TestPostgresSnapshotViewOutsideSearchPath(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	viewName := "Quarterly.Report View"
	postgresExec(t, s, "CREATE VIEW "+schema+"."+pq.QuoteIdentifier(viewName)+" AS SELECT 1 AS "+pq.QuoteIdentifier("View Value"))

	snapshot := postgresSnapshot(t, s)
	view, ok := snapshot.Views[s.schemas[0]+`."Quarterly.Report View"`]
	if !ok {
		t.Fatalf("quoted view %q missing from snapshot", viewName)
	}
	if !strings.Contains(view.Definition, `"View Value"`) {
		t.Fatalf("quoted view definition = %q, want quoted column", view.Definition)
	}
}

func TestPostgresSnapshotMaterializedViewOutsideSearchPath(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	viewName := "Materialized.Report View"
	postgresExec(t, s, "CREATE MATERIALIZED VIEW "+schema+"."+pq.QuoteIdentifier(viewName)+" AS SELECT 1 AS "+pq.QuoteIdentifier("Materialized Value"))

	snapshot := postgresSnapshot(t, s)
	view, ok := snapshot.MatViews[s.schemas[0]+`."Materialized.Report View"`]
	if !ok {
		t.Fatalf("quoted materialized view %q missing from snapshot", viewName)
	}
	if !strings.Contains(view.Definition, `"Materialized Value"`) {
		t.Fatalf("quoted materialized view definition = %q, want quoted column", view.Definition)
	}
	if !view.IsPopulated {
		t.Fatalf("quoted materialized view %q is not populated", viewName)
	}
}
