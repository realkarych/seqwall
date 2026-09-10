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
	table, ok := snapshot.Tables[tableName]
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
	columns := snapshot.Tables["typed_rows"].Columns
	if len(columns) != 1 {
		t.Fatalf("typed_rows has %d columns, want 1: %+v", len(columns), columns)
	}
}

func TestPostgresSnapshotViewOutsideSearchPath(t *testing.T) {
	s := newPostgresIntegrationWorker(t, 1)
	schema := pq.QuoteIdentifier(s.schemas[0])
	viewName := "Quarterly.Report View"
	postgresExec(t, s, "CREATE VIEW "+schema+"."+pq.QuoteIdentifier(viewName)+" AS SELECT 1 AS "+pq.QuoteIdentifier("View Value"))

	snapshot := postgresSnapshot(t, s)
	view, ok := snapshot.Views[viewName]
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
	view, ok := snapshot.MatViews[viewName]
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
