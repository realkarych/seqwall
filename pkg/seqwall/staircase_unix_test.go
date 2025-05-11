//go:build !windows
// +build !windows

package seqwall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCalculateStairDepth(t *testing.T) {
	t.Parallel()

	migs := []string{"1.sql", "2.sql", "3.sql", "4.sql", "5.sql"}
	cases := []struct {
		name  string
		depth int
		want  int
	}{
		{"depth 0 -> all", 0, 5},
		{"depth less than len", 3, 3},
		{"depth bigger than len", 10, 5},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := &StaircaseWorker{depth: c.depth}
			got := w.calculateStairDepth(migs)
			if got != c.want {
				t.Fatalf("calculateStairDepth() got %d, want %d", got, c.want)
			}
		})
	}
}

func TestBuildSchemaCond(t *testing.T) {
	t.Parallel()

	cases := []struct {
		schemas []string
		col     string
		want    string
	}{
		{nil, "table_schema", "table_schema = 'public'"},
		{[]string{"public"}, "table_schema", "table_schema = 'public'"},
		{[]string{"public", "extra"}, "tc.table_schema", "tc.table_schema IN ('public', 'extra')"},
	}

	for _, c := range cases {
		c := c
		name := strings.Join(c.schemas, "+")
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := &StaircaseWorker{schemas: c.schemas}
			got := w.buildSchemaCond(c.col)
			if got != c.want {
				t.Fatalf("buildSchemaCond() got %q, want %q", got, c.want)
			}
		})
	}
}

func TestExecuteCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "ok.sh")
	body := "#!/bin/sh\necho OK"
	if err := os.WriteFile(scriptPath, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	w := &StaircaseWorker{}

	out, err := w.executeCommand(scriptPath, "dummy")
	if err != nil {
		t.Fatalf("executeCommand() unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Fatalf("executeCommand() output = %q, want 'OK'", out)
	}
	failPath := filepath.Join(dir, "fail.sh")
	bodyFail := "#!/bin/sh\nexit 42"
	if err := os.WriteFile(failPath, []byte(bodyFail), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	_, err = w.executeCommand(failPath, "dummy")
	if err == nil {
		t.Fatalf("executeCommand() expected error, got nil")
	}
}
