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

func makeConstraint(table, name, consType, defStr string, valid bool) driver.ConstraintDefinition {
	return driver.ConstraintDefinition{
		TableName:      table,
		ConstraintType: consType,
		Definition:     sql.NullString{String: defStr, Valid: valid},
	}
}

func TestNormalizeConstraints(t *testing.T) {
	src := map[string]driver.ConstraintDefinition{
		"c_not_null": makeConstraint("users", "c_not_null", "CHECK", "email IS NOT NULL", true),
		"c_check":    makeConstraint("users", "c_check", "CHECK", "age > 0", true),
		"c_other":    makeConstraint("users", "c_other", "UNIQUE", "(id)", true),
	}
	res := normalizeConstraints(src)

	// CHECK IS NOT NULL should be renamed to users_email_not_null
	newKey := "users_email_not_null"
	if _, ok := res[newKey]; !ok {
		t.Errorf("expected renamed constraint key %q, got keys %v", newKey, keys(res))
	}
	// original key should be removed
	if _, ok := res["c_not_null"]; ok {
		t.Error("original NOT NULL constraint key should be removed")
	}
	// other constraints should remain
	if _, ok := res["c_check"]; !ok {
		t.Error("expected CHECK constraint without rename to be kept")
	}
	if _, ok := res["c_other"]; !ok {
		t.Error("expected non-CHECK constraint to be kept")
	}
}

// helper to list map keys
func keys(m map[string]driver.ConstraintDefinition) []string {
	res := make([]string, 0, len(m))
	for k := range m {
		res = append(res, k)
	}
	return res
}

func TestCompareSchemas_NoDifferences(t *testing.T) {
	before := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeConstraint("t1", "c1", "CHECK", "col IS NOT NULL", true),
	}}
	after := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeConstraint("t1", "c1", "CHECK", "col IS NOT NULL", true),
	}}
	err := compareSchemas(before, after)
	if err != nil {
		t.Errorf("expected no diff error, got %v", err)
	}
}

func TestCompareSchemas_WithDifferences(t *testing.T) {
	before := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeConstraint("t1", "c1", "CHECK", "col > 0", true),
	}}
	after := &driver.SchemaSnapshot{Constraints: map[string]driver.ConstraintDefinition{
		"c1": makeConstraint("t1", "c1", "CHECK", "col >= 0", true),
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
