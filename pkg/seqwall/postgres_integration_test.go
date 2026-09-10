package seqwall

import (
	"crypto/rand"
	"fmt"
	"os"
	"testing"

	"github.com/lib/pq"
	"github.com/realkarych/seqwall/pkg/driver"
)

func newPostgresIntegrationWorker(t *testing.T, schemaCount int) *StaircaseWorker {
	t.Helper()
	dsn := os.Getenv("SEQWALL_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("SEQWALL_TEST_POSTGRES_URL is not set")
	}
	client, err := driver.NewPostgresClient(dsn)
	if err != nil {
		t.Fatalf("connect to integration PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close integration PostgreSQL: %v", err)
		}
	})
	s := &StaircaseWorker{dbClient: client}
	for range schemaCount {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			t.Fatalf("generate schema name: %v", err)
		}
		schema := fmt.Sprintf("seqwall_test_%x", suffix)
		postgresExec(t, s, "CREATE SCHEMA "+pq.QuoteIdentifier(schema))
		s.schemas = append(s.schemas, schema)
		t.Cleanup(func() {
			if _, err := client.Execute("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE"); err != nil {
				t.Errorf("drop integration schema %q: %v", schema, err)
			}
		})
	}
	return s
}

func postgresExec(t *testing.T, s *StaircaseWorker, query string) {
	t.Helper()
	if _, err := s.dbClient.Execute(query); err != nil {
		t.Fatalf("execute integration SQL: %v", err)
	}
}

func postgresSnapshot(t *testing.T, s *StaircaseWorker) *driver.SchemaSnapshot {
	t.Helper()
	snapshot, err := s.makeSchemaSnapshot()
	if err != nil {
		t.Fatalf("snapshot integration schema: %v", err)
	}
	return snapshot
}
