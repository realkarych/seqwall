package seqwall

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidConfig(t *testing.T) {
	validYAML := `
default:
  database: postgres
  migrations_dir: path/to/migrations
  migrate_up: "make up"
  migrate_down: "make down"
  test_depth: 3
  env:
    path: path/to/.env
    db_url: POSTGRES_URL
staging:
  database: postgres
  migrations_dir: path/to/migrations
  migrate_up: "make up"
  migrate_down: "make down"
  test_depth: 3
  env:
    path: path/to/.env
    db_url: POSTGRES_URL
`
	parser := NewYamlParser()
	cfg, err := parser.Parse(strings.NewReader(validYAML))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expectedConfig := SeqwallConfig{
		Database:      Postgres,
		MigrationsDir: "path/to/migrations",
		MigrateUp:     "make up",
		MigrateDown:   "make down",
		TestDepth:     3,
		Env: EnvConfig{
			Path:  "path/to/.env",
			DbUrl: "POSTGRES_URL",
		},
	}

	if len(cfg) != 2 {
		t.Errorf("expected 2 environments, got: %d", len(cfg))
	}

	for _, env := range []string{"default", "staging"} {
		conf, ok := cfg[env]
		if !ok {
			t.Errorf("expected environment %q to be present", env)
			continue
		}
		if !reflect.DeepEqual(conf, expectedConfig) {
			t.Errorf("configuration for %q does not match expected value.\nGot:      %+v\nExpected: %+v", env, conf, expectedConfig)
		}
	}
}

func TestInvalidDatabase(t *testing.T) {
	invalidYAML := `
default:
  database: mysql
  migrations_dir: path/to/migrations
  migrate_up: "make up"
  migrate_down: "make down"
  test_depth: 3
  env:
    path: path/to/.env
    db_url: POSTGRES_URL
`
	parser := NewYamlParser()
	_, err := parser.Parse(strings.NewReader(invalidYAML))
	if err == nil {
		t.Fatal("expected error due to invalid database, got none")
	}

	expectedSubstring := "invalid database provided"
	if !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("expected error message to contain %q, got: %s", expectedSubstring, err.Error())
	}
}

func TestInvalidYAML(t *testing.T) {
	invalidYAML := "this is not yaml"
	parser := NewYamlParser()
	_, err := parser.Parse(strings.NewReader(invalidYAML))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got none")
	}
}
