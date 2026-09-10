package seqwall

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/realkarych/seqwall/pkg/driver"
)

func TestMarshalSnapshot(t *testing.T) {
	snap := &driver.SchemaSnapshot{
		Constraints: map[string]driver.ConstraintDefinition{
			"c1": {TableName: "tbl1", ConstraintType: "CHK", Definition: sql.NullString{String: "foo", Valid: true}},
		},
	}
	b, err := marshalSnapshot(snap)
	if err != nil {
		t.Fatalf("marshalSnapshot error: %v", err)
	}
	var out driver.SchemaSnapshot
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("invalid JSON generated: %v", err)
	}
	if len(out.Constraints) != len(snap.Constraints) {
		t.Errorf("expected %d constraints, got %d", len(snap.Constraints), len(out.Constraints))
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 3 {
		t.Fatalf("unexpected JSON formatting: %s", string(b))
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Errorf("expected indentation in JSON, got: %q", lines[1])
	}
}

func TestDiffJSON(t *testing.T) {
	a := []byte("line1\nline2\nline3\n")
	b := []byte("line1\nlineX\nline3\n")
	d, err := diffJSON(a, b)
	if err != nil {
		t.Fatalf("diffJSON error: %v", err)
	}
	if !strings.Contains(d, "Snapshot Before") {
		t.Error("diff header missing 'Snapshot Before'")
	}
	if !strings.Contains(d, "-line2") || !strings.Contains(d, "+lineX") {
		t.Errorf("unexpected diff result: %s", d)
	}
}

func makeCheckConstraint(table, defStr string) driver.ConstraintDefinition {
	return driver.ConstraintDefinition{
		TableName:      table,
		ConstraintType: "CHECK",
		Definition:     sql.NullString{String: defStr, Valid: true},
	}
}

func TestCompareSchemas_NoDifferences(t *testing.T) {
	before := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeCheckConstraint("t1", "col IS NOT NULL"),
	}}
	after := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeCheckConstraint("t1", "col IS NOT NULL"),
	}}
	err := compareSchemas(before, after)
	if err != nil {
		t.Errorf("expected no diff error, got %v", err)
	}
}

func TestCompareSchemas_WithDifferences(t *testing.T) {
	before := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeCheckConstraint("t1", "col > 0"),
	}}
	after := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeCheckConstraint("t1", "col >= 0"),
	}}
	err := compareSchemas(before, after)
	if err == nil {
		t.Fatal("expected error for differing snapshots, got nil")
	}
	if !errors.Is(err, ErrSnapshotsDiffer()) {
		t.Errorf("expected ErrSnapshotsDiffer, got %v", err)
	}
	if !strings.Contains(err.Error(), "Snapshot Before") || !strings.Contains(err.Error(), "Snapshot After") {
		t.Errorf("diff header missing in error: %v", err)
	}
}

func TestCompareSchemasDoesNotMutateInputs(t *testing.T) {
	before := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"first_real_key": makeCheckConstraint("items", "x IS NOT NULL"),
	}}
	after := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"second_real_key": makeCheckConstraint("items", "x IS NOT NULL"),
	}}
	beforeBytes, err := marshalSnapshot(before)
	if err != nil {
		t.Fatalf("marshal before input: %v", err)
	}
	afterBytes, err := marshalSnapshot(after)
	if err != nil {
		t.Fatalf("marshal after input: %v", err)
	}

	firstErr := compareSchemas(before, after)
	secondErr := compareSchemas(before, after)
	if !errors.Is(firstErr, ErrSnapshotsDiffer()) || !errors.Is(secondErr, ErrSnapshotsDiffer()) {
		t.Fatalf("comparison errors = (%v, %v), want ErrSnapshotsDiffer twice", firstErr, secondErr)
	}
	if firstErr.Error() != secondErr.Error() {
		t.Fatalf("repeated comparison changed result:\nfirst: %v\nsecond: %v", firstErr, secondErr)
	}

	gotBeforeBytes, err := marshalSnapshot(before)
	if err != nil {
		t.Fatalf("marshal compared before input: %v", err)
	}
	gotAfterBytes, err := marshalSnapshot(after)
	if err != nil {
		t.Fatalf("marshal compared after input: %v", err)
	}
	if string(gotBeforeBytes) != string(beforeBytes) {
		t.Fatalf("before input changed:\nbefore comparison:\n%s\nafter comparison:\n%s", beforeBytes, gotBeforeBytes)
	}
	if string(gotAfterBytes) != string(afterBytes) {
		t.Fatalf("after input changed:\nbefore comparison:\n%s\nafter comparison:\n%s", afterBytes, gotAfterBytes)
	}
}
