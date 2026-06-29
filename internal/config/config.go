package config

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"regexp"
	"strings"

	"foghorn/internal/duration"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ptrTo returns a pointer to a copy of v. Using a function avoids shared
// mutable state when the same default value is needed in multiple places.
func ptrTo[T any](v T) *T { return &v }

// defaultSilenceEditorMatchers returns a fresh slice of the default
// always-visible matchers. A function (not a package-level var) is used so
// that each caller gets an independent slice with no aliasing.
func defaultSilenceEditorMatchers() []string {
	return []string{"alertname", "cluster", "severity", "pod"}
}

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
			Theme:             "system",
			PopupWidth:        800,
			PopupHeight:       600,
			PopupPosition:     "top_right",
			AutoPosition:      ptrTo(true),
			DefaultCreatedBy:  defaultCreatedBy(),
			AlwaysOnTop:       ptrTo(true),
			PopupFollowCursor: ptrTo(true),
			Scale: UIScale{
				Factor:       1.0,
				Mode:         "fonts",
				ApplyToPopup: true,
			},
			SilenceEditor: SilenceEditorConfig{
				AlwaysVisibleMatchers: ptrTo(defaultSilenceEditorMatchers()),
				CollapseMatchers:      ptrTo(true),
			},
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

	cfg := *Default()
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
		if src.URL == "" && !strings.EqualFold(src.Type, "betterstack") {
			return fmt.Errorf("source[%d] %q: url is required", i, src.Name)
		}
		if src.URL == "" && strings.EqualFold(src.Type, "betterstack") {
			cfg.Sources[i].URL = "https://uptime.betterstack.com"
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
	rawPopupPosition := cfg.UI.PopupPosition
	normalizedPopupPosition := strings.ToLower(strings.TrimSpace(rawPopupPosition))
	switch normalizedPopupPosition {
	case "", "top_right", "top-right":
		cfg.UI.PopupPosition = "top_right"
	case "top_left", "top-left":
		cfg.UI.PopupPosition = "top_left"
	case "bottom_right", "bottom-right":
		cfg.UI.PopupPosition = "bottom_right"
	case "bottom_left", "bottom-left":
		cfg.UI.PopupPosition = "bottom_left"
	default:
		if normalizedPopupPosition != rawPopupPosition {
			return fmt.Errorf("ui.popup_position %q (normalized: %q) must be one of top_right, top_left, bottom_right, bottom_left", rawPopupPosition, normalizedPopupPosition)
		}
		return fmt.Errorf("ui.popup_position %q must be one of top_right, top_left, bottom_right, bottom_left", rawPopupPosition)
	}
	if cfg.Notifications.BatchThreshold == 0 {
		cfg.Notifications.BatchThreshold = 5
	}
	if strings.TrimSpace(cfg.UI.DefaultCreatedBy) == "" {
		cfg.UI.DefaultCreatedBy = defaultCreatedBy()
	}
	if cfg.UI.SilenceEditor.AlwaysVisibleMatchers == nil {
		cfg.UI.SilenceEditor.AlwaysVisibleMatchers = ptrTo(defaultSilenceEditorMatchers())
	}
	if cfg.UI.SilenceEditor.CollapseMatchers == nil {
		cfg.UI.SilenceEditor.CollapseMatchers = ptrTo(true)
	}
	if cfg.UI.AlwaysOnTop == nil {
		cfg.UI.AlwaysOnTop = ptrTo(true)
	}
	if cfg.UI.AutoPosition == nil {
		cfg.UI.AutoPosition = ptrTo(true)
	}
	if cfg.UI.PopupFollowCursor == nil {
		cfg.UI.PopupFollowCursor = ptrTo(true)
	}
	if err := normalizeUIScale(&cfg.UI.Scale); err != nil {
		return err
	}
	for i := range cfg.Hide {
		rule := &cfg.Hide[i]
		if len(rule.Matchers) == 0 {
			return fmt.Errorf("hide[%d]: at least one matcher is required", i)
		}
		parsed, err := duration.Parse(rule.MinAge)
		if err != nil {
			return fmt.Errorf("hide[%d] min_age: invalid duration %q: %w", i, rule.MinAge, err)
		}
		if parsed < 0 {
			return fmt.Errorf("hide[%d] min_age: %q must be non-negative", i, rule.MinAge)
		}
		rule.ParsedMinAge = parsed
	}
	if err := cfg.Display.finalizeVisibleEntries(); err != nil {
		return err
	}
	return nil
}

func normalizeUIScale(scale *UIScale) error {
	if scale.Factor == 0 {
		scale.Factor = 1.0
	}
	if scale.Factor < 0.75 {
		log.Printf("config: ui.scale.factor %.2f is outside [0.75, 2.0], clamped to 0.75", scale.Factor)
		scale.Factor = 0.75
	}
	if scale.Factor > 2.0 {
		log.Printf("config: ui.scale.factor %.2f is outside [0.75, 2.0], clamped to 2.0", scale.Factor)
		scale.Factor = 2.0
	}

	mode := strings.ToLower(strings.TrimSpace(scale.Mode))
	switch mode {
	case "":
		scale.Mode = "fonts"
	case "fonts", "interface":
		scale.Mode = mode
	default:
		return fmt.Errorf("ui.scale.mode %q must be one of fonts, interface", scale.Mode)
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
