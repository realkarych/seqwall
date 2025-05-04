package seqwall

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/realkarych/seqwall/pkg/driver"
)

const (
	diffContextLines = 3
)

func marshalSnapshot(snap *driver.SchemaSnapshot) ([]byte, error) {
	return json.MarshalIndent(snap, "", "  ")
}

func diffJson(a, b []byte) (string, error) {
	d := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(a)),
		B:        difflib.SplitLines(string(b)),
		FromFile: "Snapshot Before",
		ToFile:   "Snapshot After",
		Context:  diffContextLines,
	}
	return difflib.GetUnifiedDiffString(d)
}

func compareSchemas(before, after *driver.SchemaSnapshot) error {
	before.Constraints = normalizeConstraints(before.Constraints)
	after.Constraints = normalizeConstraints(after.Constraints)
	b, err := marshalSnapshot(before)
	if err != nil {
		return fmt.Errorf("marshal before: %w", err)
	}
	a, err := marshalSnapshot(after)
	if err != nil {
		return fmt.Errorf("marshal after: %w", err)
	}
	out, err := diffJson(b, a)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	if out != "" {
		return fmt.Errorf("%w:\n%s", ErrSnapshotsDiffer(), out)
	}
	return nil
}

func normalizeConstraints(src map[string]driver.ConstraintDefinition) map[string]driver.ConstraintDefinition {
	checkNullConstraintSubmatchCount := 2
	res := make(map[string]driver.ConstraintDefinition)
	re := regexp.MustCompile(`^([A-Za-z0-9_]+)\s+IS\s+NOT\s+NULL$`)
	for _, c := range src {
		if c.ConstraintType == "CHECK" && c.Definition.Valid {
			if m := re.FindStringSubmatch(c.Definition.String); len(m) == checkNullConstraintSubmatchCount {
				k := c.TableName + "_" + m[1] + "_not_null"
				res[k] = c
				continue
			}
		}
	}
	for k, c := range src {
		if c.ConstraintType == "CHECK" && c.Definition.Valid && re.MatchString(c.Definition.String) {
			continue
		}
		res[k] = c
	}
	return res
}
