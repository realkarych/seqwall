package main

import (
	"flag"
	"os"
	"testing"
)

func TestParseConfigDefault(t *testing.T) {
	yamlContent := `
default:
  database: "postgres"
  migrations_dir: "path/to/migrations"
  migrate_up: "make up"
  migrate_down: "make down"
  test_depth: 3
  env:
    path: "path/to/.env"
    db_url: "POSTGRES_URL"
`
	tmpfile, err := os.CreateTemp("", "seqwall.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"cmd", "--config", tmpfile.Name(), "--env", "default"}

	config, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if config.Database != "postgres" {
		t.Errorf("expected database 'postgres', got %s", config.Database)
	}
	if config.MigrationsDir != "path/to/migrations" {
		t.Errorf("expected migrations_dir 'path/to/migrations', got %s", config.MigrationsDir)
	}
	if config.MigrateUp != "make up" {
		t.Errorf("expected migrate_up 'make up', got %s", config.MigrateUp)
	}
	if config.MigrateDown != "make down" {
		t.Errorf("expected migrate_down 'make down', got %s", config.MigrateDown)
	}
	if config.TestDepth != 3 {
		t.Errorf("expected test_depth 3, got %d", config.TestDepth)
	}
	if config.Env.Path != "path/to/.env" {
		t.Errorf("expected env.path 'path/to/.env', got %s", config.Env.Path)
	}
	if config.Env.DbUrl != "POSTGRES_URL" {
		t.Errorf("expected env.db_url 'POSTGRES_URL', got %s", config.Env.DbUrl)
	}
}

func TestParseConfigCustom(t *testing.T) {
	yamlContent := `
default:
  database: "postgres"
  migrations_dir: "path/to/migrations"
  migrate_up: "make up"
  migrate_down: "make down"
  test_depth: 3
  env:
    path: "path/to/.env"
    db_url: "POSTGRES_URL"
custom:
  database: "postgres"
  migrations_dir: "custom/migrations"
  migrate_up: "custom up"
  migrate_down: "custom down"
  test_depth: 5
  env:
    path: "custom/path"
    db_url: "CUSTOM_URL"
`
	tmpfile, err := os.CreateTemp("", "seqwall.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"cmd", "--config", tmpfile.Name(), "--env", "custom"}

	config, err := parseConfig()
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if config.Database != "postgres" {
		t.Errorf("expected database 'postgres', got %s", config.Database)
	}
	if config.MigrationsDir != "custom/migrations" {
		t.Errorf("expected migrations_dir 'custom/migrations', got %s", config.MigrationsDir)
	}
	if config.MigrateUp != "custom up" {
		t.Errorf("expected migrate_up 'custom up', got %s", config.MigrateUp)
	}
	if config.MigrateDown != "custom down" {
		t.Errorf("expected migrate_down 'custom down', got %s", config.MigrateDown)
	}
	if config.TestDepth != 5 {
		t.Errorf("expected test_depth 5, got %d", config.TestDepth)
	}
	if config.Env.Path != "custom/path" {
		t.Errorf("expected env.path 'custom/path', got %s", config.Env.Path)
	}
	if config.Env.DbUrl != "CUSTOM_URL" {
		t.Errorf("expected env.db_url 'CUSTOM_URL', got %s", config.Env.DbUrl)
	}
}
