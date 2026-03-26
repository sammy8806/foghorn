package config

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Default returns a minimal usable config with no sources.
func Default() *Config {
	return &Config{
		Severities: DefaultSeverityConfig(),
		Notifications: NotificationsConfig{
			Enabled:        true,
			OnNew:          true,
			OnResolved:     false,
			BatchThreshold: 5,
		},
		UI: UIConfig{
			Theme:            "system",
			PopupWidth:       800,
			PopupHeight:      600,
			DefaultCreatedBy: defaultCreatedBy(),
		},
	}
}

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
	normalizedSeverities, err := NormalizeSeverityConfig(cfg.Severities)
	if err != nil {
		return err
	}
	cfg.Severities = SeverityConfig{
		Default: normalizedSeverities.Default,
		Levels:  make([]SeverityLevel, 0, len(normalizedSeverities.Levels)),
	}
	for _, level := range normalizedSeverities.Levels {
		cfg.Severities.Levels = append(cfg.Severities.Levels, SeverityLevel{
			Name:    level.Name,
			Color:   level.Color,
			Aliases: level.Aliases,
		})
	}

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
		if strings.TrimSpace(src.SeverityLabel) == "" {
			cfg.Sources[i].SeverityLabel = "severity"
		}
	}
	if cfg.UI.PopupWidth == 0 {
		cfg.UI.PopupWidth = 800
	}
	if cfg.UI.PopupHeight == 0 {
		cfg.UI.PopupHeight = 600
	}
	if cfg.Notifications.BatchThreshold == 0 {
		cfg.Notifications.BatchThreshold = 5
	}
	if strings.TrimSpace(cfg.UI.DefaultCreatedBy) == "" {
		cfg.UI.DefaultCreatedBy = defaultCreatedBy()
	}
	return nil
}

func CurrentUsername() string {
	if current, err := user.Current(); err == nil {
		if value := strings.TrimSpace(current.Username); value != "" {
			log.Printf("config: resolved current username via os/user: %q", value)
			return value
		}
		log.Printf("config: os/user returned empty username")
	} else {
		log.Printf("config: os/user lookup failed: %v", err)
	}
	for _, envKey := range []string{"USER", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(envKey)); value != "" {
			log.Printf("config: resolved current username via env %s: %q", envKey, value)
			return value
		}
	}
	log.Printf("config: falling back to default username %q", "foghorn")
	return "foghorn"
}

func defaultCreatedBy() string {
	return CurrentUsername()
}

func ResolveCreatedByDefault(configured string) string {
	if value := strings.TrimSpace(configured); value != "" {
		log.Printf("config: using configured default_created_by: %q", value)
		return value
	}
	return CurrentUsername()
}
