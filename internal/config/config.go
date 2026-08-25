package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/user"
	"regexp"
	"sort"
	"strings"
	"time"

	"foghorn/internal/duration"

	"gopkg.in/yaml.v3"
)

// DefaultSourceTimeout bounds a single source's fetch (alerts + silences +
// on-call) when a source does not set its own `timeout`. It matches the
// providers' built-in HTTP client timeout so a slow/broken source cannot hang
// a poll indefinitely.
const DefaultSourceTimeout = 10 * time.Second

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

// Load reads and parses a config file, expanding environment variables. A
// non-nil error means the file could not be read or parsed at all — every
// other problem is reported as a Diagnostic against a config that is still
// usable, with the offending entries dropped.
func Load(path string) (*Config, Diagnostics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading config: %w", err)
	}

	expanded := expandEnvVars(string(data))

	cfg := *Default()
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, nil, fmt.Errorf("parsing config: %w", err)
	}

	var diags Diagnostics
	if err := validate(&cfg, &diags); err != nil {
		return nil, nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, diags, nil
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

// warnInsecureSourceURL logs a warning for sources fetched over plain HTTP to a
// non-local host. Alert payloads drive links, notifications and configured
// resolver processes, and the auth credentials go out with every request —
// over cleartext HTTP anyone on the network path can read or rewrite all of it.
func warnInsecureSourceURL(name, rawURL string) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !strings.EqualFold(parsed.Scheme, "http") {
		return
	}
	if isLoopbackHost(parsed.Hostname()) {
		return
	}
	log.Printf("config: WARNING source %q uses plain HTTP (%s): credentials and alert content are sent in cleartext and can be modified in transit; use https", name, parsed.Host)
}

// warnInsecureAuthURL logs a warning for an OIDC endpoint configured as plain
// HTTP to a non-local host. A cleartext issuer lets an on-path attacker choose
// which endpoints are used, and cleartext device/token endpoints carry the
// client_id and client_secret in an HTTP Basic header.
func warnInsecureAuthURL(name, field, rawURL string) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || !strings.EqualFold(parsed.Scheme, "http") {
		return
	}
	if isLoopbackHost(parsed.Hostname()) {
		return
	}
	log.Printf("config: WARNING source %q auth.%s uses plain HTTP (%s): OIDC client credentials and tokens are sent in cleartext and can be read or modified in transit; use https", name, field, parsed.Host)
}

// warnInsecureAuthURLs checks every OIDC endpoint a source may contact.
func warnInsecureAuthURLs(src SourceConfig) {
	switch strings.ToLower(strings.TrimSpace(src.Auth.Type)) {
	case "oidc", "oidc_device":
	default:
		return
	}
	warnInsecureAuthURL(src.Name, "issuer_url", src.Auth.IssuerURL)
	warnInsecureAuthURL(src.Name, "device_authorization_url", src.Auth.DeviceAuthorizationURL)
	warnInsecureAuthURL(src.Name, "token_url", src.Auth.TokenURL)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func validate(cfg *Config, diags *Diagnostics) error {
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

	enabledSources := make([]SourceConfig, 0, len(cfg.Sources))
	for i, src := range cfg.Sources {
		if src.Enabled != nil && !*src.Enabled {
			continue
		}
		if src.Name == "" {
			diags.Drop(fmt.Sprintf("source[%d]", i), "name is required")
			continue
		}
		if src.Type == "" {
			diags.Drop(fmt.Sprintf("source[%d] %q", i, src.Name), "type is required")
			continue
		}
		if src.URL == "" && !strings.EqualFold(src.Type, "betterstack") {
			diags.Drop(fmt.Sprintf("source[%d] %q", i, src.Name), "url is required")
			continue
		}
		if src.URL == "" && strings.EqualFold(src.Type, "betterstack") {
			src.URL = "https://uptime.betterstack.com"
		}
		if src.PollInterval == 0 {
			src.PollInterval = 30_000_000_000 // 30s default
		}
		if src.Timeout <= 0 {
			src.Timeout = DefaultSourceTimeout
		}
		if strings.TrimSpace(src.SeverityLabel) == "" {
			src.SeverityLabel = "severity"
		}
		warnInsecureSourceURL(src.Name, src.URL)
		warnInsecureAuthURLs(src)
		enabledSources = append(enabledSources, src)
	}
	cfg.Sources = enabledSources
	cfg.Actions = validateActions(cfg.Actions, diags)
	cfg.Resolvers = validateResolvers(cfg.Resolvers, diags)
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

// validateActions returns the actions that are usable, recording a diagnostic
// for each one it drops. Normalization of the surviving entries happens in
// place, as before.
func validateActions(actions []ActionConfig, diags *Diagnostics) []ActionConfig {
	surviving := make([]ActionConfig, 0, len(actions))
	for i := range actions {
		action := actions[i]
		actionType := strings.ToLower(strings.TrimSpace(action.Action.Type))
		action.Action.Type = actionType
		field := fmt.Sprintf("actions[%d] %q", i, action.Name)

		switch actionType {
		case "url", "clipboard":
			if strings.TrimSpace(action.Action.Template) == "" {
				diags.Drop(field, "%s action requires a template", actionType)
				continue
			}
		case "shell":
			diags.Drop(field, "shell actions are no longer supported because remote alert fields could become shell syntax; use a url or clipboard action")
			continue
		case "":
			diags.Drop(field, "action type is required")
			continue
		default:
			diags.Drop(field, "unsupported action type %q; use url or clipboard", actionType)
			continue
		}
		surviving = append(surviving, action)
	}
	return surviving
}

// validateResolvers returns the resolvers that are usable, recording a
// diagnostic for each one it drops. Dropping is safe: resolve.Engine is
// additive, so a missing resolver means the UI renders the raw field value.
func validateResolvers(resolvers []ResolverConfig, diags *Diagnostics) []ResolverConfig {
	surviving := make([]ResolverConfig, 0, len(resolvers))
	for i := range resolvers {
		resolver := resolvers[i]
		resolver.Field = strings.TrimSpace(resolver.Field)
		resolver.Command = strings.TrimSpace(resolver.Command)
		resolver.Stdin = strings.ToLower(strings.TrimSpace(resolver.Stdin))
		field := fmt.Sprintf("resolvers[%d] %q", i, resolver.Name)

		if resolver.Field == "" {
			diags.Drop(field, "field is required")
			continue
		}
		if resolver.Command == "" {
			diags.Drop(field, "command is required")
			continue
		}
		if hasProcessTemplate(resolver.Command) {
			diags.Drop(field, "%s", resolverTemplateMessage("command"))
			continue
		}
		templated := false
		for argIndex, arg := range resolver.Args {
			if hasProcessTemplate(arg) {
				diags.Drop(field, "%s", resolverTemplateMessage(fmt.Sprintf("args[%d]", argIndex)))
				templated = true
				break
			}
		}
		if templated {
			continue
		}

		envKeys := make([]string, 0, len(resolver.Env))
		for key := range resolver.Env {
			envKeys = append(envKeys, key)
		}
		sort.Strings(envKeys)
		for _, key := range envKeys {
			if hasProcessTemplate(resolver.Env[key]) {
				diags.Drop(field, "%s", resolverTemplateMessage(fmt.Sprintf("env.%s", key)))
				templated = true
				break
			}
		}
		if templated {
			continue
		}

		switch resolver.Stdin {
		case "value", "json":
		case "":
			diags.Drop(field, "stdin is required; use stdin: value for the selected field or stdin: json for structured alert data")
			continue
		default:
			diags.Drop(field, "stdin %q must be value or json", resolver.Stdin)
			continue
		}
		surviving = append(surviving, resolver)
	}
	return surviving
}

func hasProcessTemplate(value string) bool {
	return strings.Contains(value, "{{") || strings.Contains(value, "}}")
}

func resolverTemplateMessage(field string) string {
	return fmt.Sprintf("templates in %s are no longer supported; use stdin: value or stdin: json and update the resolver program to read stdin", field)
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
