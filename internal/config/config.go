package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Load reads and parses a config file, expanding environment variables.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func expandEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match
	})
}

func validate(cfg *Config) error {
	for i, src := range cfg.Sources {
		if src.Name == "" {
			return fmt.Errorf("source[%d]: name is required", i)
		}
		if src.Type == "" {
			return fmt.Errorf("source[%d] %q: type is required", i, src.Name)
		}
		if src.URL == "" {
			return fmt.Errorf("source[%d] %q: url is required", i, src.Name)
		}
		if src.PollInterval == 0 {
			cfg.Sources[i].PollInterval = 30_000_000_000 // 30s default
		}
	}
	if cfg.UI.PopupWidth == 0 {
		cfg.UI.PopupWidth = 800
	}
	if cfg.UI.PopupHeight == 0 {
		cfg.UI.PopupHeight = 600
	}
	return nil
}
