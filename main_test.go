package main

import (
	"bytes"
	"runtime/debug"
	"strings"
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

func TestStaircaseHelpDescribesFlagsAndDefaults(t *testing.T) {
	tests := []struct {
		name       string
		usage      string
		defaultVal string
	}{
		{name: "postgres-url", usage: "PostgreSQL connection URL (defaults to DATABASE_URL)", defaultVal: ""},
		{name: "migrations-path", usage: "Directory containing lexicographically ordered migration files", defaultVal: ""},
		{name: "upgrade", usage: "Command that applies exactly one migration", defaultVal: ""},
		{name: "downgrade", usage: "Command that reverts exactly one migration", defaultVal: ""},
		{name: "test-snapshots", usage: "Compare schema snapshots", defaultVal: "true"},
		{name: "schema", usage: "Schema to include in testing (repeatable)", defaultVal: "[public]"},
		{name: "depth", usage: "Number of migrations to test (0 means all)", defaultVal: "0"},
		{name: "migrations-extension", usage: "Migration filename extension", defaultVal: ".sql"},
	}

	cmd := newStaircaseCmd(&StaircaseOptions{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("render staircase help: %v", err)
	}
	help := output.String()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("flag %q not found", tt.name)
			}
			if flag.Usage != tt.usage {
				t.Errorf("flag %q usage = %q, want %q", tt.name, flag.Usage, tt.usage)
			}
			if flag.DefValue != tt.defaultVal {
				t.Errorf("flag %q default = %q, want %q", tt.name, flag.DefValue, tt.defaultVal)
			}
			if !strings.Contains(help, "--"+tt.name) || !strings.Contains(help, tt.usage) {
				t.Errorf("rendered help does not describe --%s:\n%s", tt.name, help)
			}
		})
	}
}

func TestStaircaseRejectsInvalidInputBeforeRun(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "positional argument",
			args:    append(validStaircaseArgs(), "unexpected"),
			wantErr: `unknown command "unexpected" for "seqwall staircase"`,
		},
		{
			name:    "negative depth",
			args:    append(validStaircaseArgs(), "--depth=-1"),
			wantErr: "--depth must be zero or greater",
		},
		{
			name:    "empty migrations path",
			args:    replaceStaircaseArg(validStaircaseArgs(), "--migrations-path=./migrations", "--migrations-path="),
			wantErr: "--migrations-path must not be empty",
		},
		{
			name:    "whitespace migrations path",
			args:    replaceStaircaseArg(validStaircaseArgs(), "--migrations-path=./migrations", "--migrations-path=  \t"),
			wantErr: "--migrations-path must not be empty",
		},
		{
			name:    "empty upgrade command",
			args:    replaceStaircaseArg(validStaircaseArgs(), "--upgrade=apply one", "--upgrade="),
			wantErr: "--upgrade must not be empty",
		},
		{
			name:    "whitespace upgrade command",
			args:    replaceStaircaseArg(validStaircaseArgs(), "--upgrade=apply one", "--upgrade=  \t"),
			wantErr: "--upgrade must not be empty",
		},
		{
			name:    "empty downgrade command",
			args:    replaceStaircaseArg(validStaircaseArgs(), "--downgrade=revert one", "--downgrade="),
			wantErr: "--downgrade must not be empty",
		},
		{
			name:    "whitespace downgrade command",
			args:    replaceStaircaseArg(validStaircaseArgs(), "--downgrade=revert one", "--downgrade=  \t"),
			wantErr: "--downgrade must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := false
			_, err := executeStaircase(t, tt.args, &run)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if run {
				t.Fatal("RunE reached for invalid input")
			}
		})
	}
}

func TestStaircaseAcceptsValidInput(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		databaseURL string
		wantURL     string
	}{
		{name: "zero depth", args: append(validStaircaseArgs(), "--depth=0"), wantURL: "postgres://flag"},
		{name: "positive depth", args: append(validStaircaseArgs(), "--depth=3"), wantURL: "postgres://flag"},
		{
			name: "internal command whitespace",
			args: replaceStaircaseArg(
				replaceStaircaseArg(validStaircaseArgs(), "--upgrade=apply one", "--upgrade=apply exactly one migration"),
				"--downgrade=revert one",
				"--downgrade=revert exactly one migration",
			),
			wantURL: "postgres://flag",
		},
		{
			name:        "environment URL fallback",
			args:        replaceStaircaseArg(validStaircaseArgs(), "--postgres-url=postgres://flag", "--postgres-url="),
			databaseURL: "postgres://environment",
			wantURL:     "postgres://environment",
		},
		{
			name:        "explicit URL precedence",
			args:        validStaircaseArgs(),
			databaseURL: "postgres://environment",
			wantURL:     "postgres://flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tt.databaseURL)
			run := false
			opts, err := executeStaircase(t, tt.args, &run)
			if err != nil {
				t.Fatalf("execute staircase: %v", err)
			}
			if !run {
				t.Fatal("RunE was not reached for valid input")
			}
			if opts.PostgresURL != tt.wantURL {
				t.Errorf("PostgresURL = %q, want %q", opts.PostgresURL, tt.wantURL)
			}
		})
	}
}

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkerVersion string
		buildVersion  string
		buildInfoOK   bool
		buildSettings []debug.BuildSetting
		want          string
	}{
		{name: "linker override", linkerVersion: "v2.0.0", buildVersion: "v1.9.0", buildInfoOK: true, want: "v2.0.0"},
		{name: "tagged module metadata", linkerVersion: "dev", buildVersion: "v1.9.0", buildInfoOK: true, want: "v1.9.0"},
		{name: "development module metadata", linkerVersion: "dev", buildVersion: "(devel)", buildInfoOK: true, want: "dev"},
		{name: "empty module metadata", linkerVersion: "dev", buildInfoOK: true, want: "dev"},
		{name: "unavailable build info", linkerVersion: "dev", buildVersion: "v1.9.0", want: "dev"},
		{
			name:          "local VCS build metadata",
			linkerVersion: "dev",
			buildVersion:  "v0.3.1-0.20260910234142-2a307eb10a01+dirty",
			buildInfoOK:   true,
			buildSettings: []debug.BuildSetting{{Key: "vcs.revision", Value: "2a307eb10a0122c16615aef86baf4dc51e2a1fb9"}},
			want:          "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readBuildInfo := func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{
					Main:     debug.Module{Version: tt.buildVersion},
					Settings: tt.buildSettings,
				}, tt.buildInfoOK
			}
			if got := resolveVersion(tt.linkerVersion, readBuildInfo); got != tt.want {
				t.Errorf("resolveVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInvalidateOptions_NoDatabaseURL(t *testing.T) {
	opts := validStaircaseOptions()
	t.Setenv("DATABASE_URL", "")
	err := invalidateOptions(opts)(nil, nil)
	expected := seqwall.ErrPostgresURLRequired()
	if err == nil || err.Error() != expected.Error() {
		t.Errorf("expected ErrPostgresURLRequired, got %v", err)
	}
}

func TestInvalidateOptions_EnvDatabaseURL(t *testing.T) {
	opts := validStaircaseOptions()
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

func validStaircaseArgs() []string {
	return []string{
		"--postgres-url=postgres://flag",
		"--migrations-path=./migrations",
		"--upgrade=apply one",
		"--downgrade=revert one",
	}
}

func executeStaircase(t *testing.T, args []string, run *bool) (*StaircaseOptions, error) {
	t.Helper()
	opts := &StaircaseOptions{}
	root := newRootCmd(opts)
	staircase, _, err := root.Find([]string{"staircase"})
	if err != nil {
		t.Fatalf("find staircase command: %v", err)
	}
	staircase.RunE = func(*cobra.Command, []string) error {
		*run = true
		return nil
	}
	root.SetArgs(append([]string{"staircase"}, args...))
	return opts, root.Execute()
}

func replaceStaircaseArg(args []string, old, replacement string) []string {
	replaced := append([]string(nil), args...)
	for i, arg := range replaced {
		if arg == old {
			replaced[i] = replacement
			return replaced
		}
	}
	return replaced
}

func validStaircaseOptions() *StaircaseOptions {
	return &StaircaseOptions{
		MigrationsPath: "./migrations",
		UpgradeCmd:     "apply one",
		DowngradeCmd:   "revert one",
	}
}
