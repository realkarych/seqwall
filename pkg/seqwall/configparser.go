package seqwall

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Database string

const (
	Postgres Database = "postgres"
)

type EnvConfig struct {
	Path  string `yaml:"path"`
	DbUrl string `yaml:"db_url"`
}

type SeqwallConfig struct {
	Database      Database  `yaml:"database"`
	MigrationsDir string    `yaml:"migrations_dir"`
	MigrateUp     string    `yaml:"migrate_up"`
	MigrateDown   string    `yaml:"migrate_down"`
	TestDepth     int       `yaml:"test_depth"`
	Env           EnvConfig `yaml:"env"`
	MigrationName string
}

type ConfigParser interface {
	Parse(r io.Reader) (map[string]SeqwallConfig, error)
}

type YamlParser struct{}

func NewYamlParser() ConfigParser {
	return &YamlParser{}
}

func (p *YamlParser) Parse(r io.Reader) (map[string]SeqwallConfig, error) {
	var cfg map[string]SeqwallConfig
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode yaml: %w", err)
	}

	for envName, conf := range cfg {
		if conf.Database != Postgres {
			return nil, fmt.Errorf("invalid database provided %s: %s. Allowed: %s", envName, conf.Database, Postgres)
		}
	}

	return cfg, nil
}
