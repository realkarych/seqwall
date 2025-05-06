package main

import (
	"os"
	"testing"

	"github.com/realkarych/seqwall/pkg/seqwall"
	"github.com/spf13/cobra"
)

func TestNewRootCmd(t *testing.T) {
	opts := &StaircaseOptions{}
	root := newRootCmd(opts)

	if root.Use != "seqwall" {
		t.Errorf("expected Use 'seqwall', got %q", root.Use)
	}

	found := false
	for _, cmd := range root.Commands() {
		if cmd.Name() == "staircase" {
			found = true
			break
		}
	}
	if !found {
		t.Error("staircase subcommand should be registered on root command")
	}
}

func TestNewStaircaseCmdFlags(t *testing.T) {
	opts := &StaircaseOptions{}
	cmd := newStaircaseCmd(opts)
	flags := cmd.Flags()

	if opts.PostgresURL != "" {
		t.Errorf("expected default PostgresURL to be empty, got %q", opts.PostgresURL)
	}
	if opts.MigrationsPath != "" || opts.UpgradeCmd != "" || opts.DowngradeCmd != "" {
		t.Error("expected default string options to be empty")
	}
	if opts.CompareSchemaSnapshots != true {
		t.Error("expected CompareSchemaSnapshots default to be true")
	}
	if len(opts.Schemas) != 1 || opts.Schemas[0] != "public" {
		t.Errorf("expected default Schemas to [public], got %v", opts.Schemas)
	}
	if opts.Depth != 0 {
		t.Errorf("expected default Depth to be 0, got %d", opts.Depth)
	}
	if opts.MigrationsExtension != ".sql" {
		t.Errorf("expected default MigrationsExtension to '.sql', got %q", opts.MigrationsExtension)
	}

	for _, name := range []string{"postgres-url", "migrations-path", "upgrade", "downgrade", "test-snapshots", "schema", "depth", "migrations-extension"} {
		if flags.Lookup(name) == nil {
			t.Errorf("flag %q not found on staircase command", name)
		}
	}

	for _, name := range []string{"migrations-path", "upgrade", "downgrade"} {
		flag := flags.Lookup(name)
		if flag == nil {
			t.Errorf("flag %q not declared", name)
			continue
		}
		if vals, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]; !ok || len(vals) == 0 || vals[0] != "true" {
			t.Errorf("flag %q should be marked as required", name)
		}
	}
}

func TestInvalidateOptions_NoDatabaseURL(t *testing.T) {
	opts := &StaircaseOptions{}
	os.Unsetenv("DATABASE_URL")
	err := invalidateOptions(opts)(nil, nil)
	expected := seqwall.ErrPostgresURLRequired()
	if err == nil || err.Error() != expected.Error() {
		t.Errorf("expected ErrPostgresURLRequired, got %v", err)
	}
}

func TestInvalidateOptions_EnvDatabaseURL(t *testing.T) {
	opts := &StaircaseOptions{}
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")

	err := invalidateOptions(opts)(nil, nil)
	if err != nil {
		t.Errorf("expected no error when DATABASE_URL is set, got %v", err)
	}
	if opts.PostgresURL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("expected PostgresURL to propagate from env, got %q", opts.PostgresURL)
	}
}

func TestMarkRequired_PanicOnMissingFlag(t *testing.T) {
	cmd := &cobra.Command{}
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when marking missing flag, got none")
		}
	}()
	markRequired(cmd, "nonexistent")
}
