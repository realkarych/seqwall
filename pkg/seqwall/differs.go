package seqwall

import (
	"encoding/json"
	"fmt"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/realkarych/seqwall/pkg/driver"
)

const (
	diffContextLines = 3
)

func marshalSnapshot(snap *driver.SchemaSnapshot) ([]byte, error) {
	return json.MarshalIndent(snap, "", "  ")
}

func diffJSON(a, b []byte) (string, error) {
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
	b, err := marshalSnapshot(before)
	if err != nil {
		return fmt.Errorf("marshal before: %w", err)
	}
	a, err := marshalSnapshot(after)
	if err != nil {
		return fmt.Errorf("marshal after: %w", err)
	}
	out, err := diffJSON(b, a)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	if out != "" {
		return fmt.Errorf("%w:\n%s", ErrSnapshotsDiffer(), out)
	}
	return nil
}
