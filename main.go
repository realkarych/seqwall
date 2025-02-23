package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/realkarych/seqwall/pkg/seqwall"
)

func main() {
	config, err := parseConfig()
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	fmt.Printf("Starts with config: %+v\n", config)
}

func parseConfig() (seqwall.SeqwallConfig, error) {
	var configPath, environment string

	flag.StringVar(&configPath, "config", "seqwall.yaml", "Path to seqwall.yaml")
	flag.StringVar(&configPath, "c", "seqwall.yaml", "Path to seqwall.yaml")
	flag.StringVar(&environment, "env", "default", "Environment in seqwall.yaml to use")
	flag.StringVar(&environment, "e", "default", "Environment in seqwall.yaml to use")

	flag.Parse()

	file, err := os.Open(configPath)
	if err != nil {
		return seqwall.SeqwallConfig{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	configs, err := seqwall.NewYamlParser().Parse(file)
	if err != nil {
		return seqwall.SeqwallConfig{}, err
	}

	conf, exists := configs[environment]
	if !exists {
		return seqwall.SeqwallConfig{}, fmt.Errorf("environment %q not found in config", environment)
	}

	return conf, nil
}
