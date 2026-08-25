package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
sources:
  - name: test-am
    type: alertmanager
    url: http://localhost:9093
    severity_label: priority
    auth:
      type: basic
      username: admin
      password: secret
    poll_interval: 30s

severities:
  default: info
  levels:
    - name: critical
      aliases: [critical, p1]
    - name: info
      color: "#00ff00"
      aliases: [info, notice]

display:
  visible_labels: [alertname, severity]
  visible_annotations: [summary]
  group_by: [cluster]
  group_by_override_key_mode: raw
  group_by_overrides:
    prod:
      - label:namespace
  priority:
    mode: after_sort
    sources: [betterstack-production]
    source_types: [betterstack]
  badges:
    - label: Ack
      field: label:status
      equals: [Acknowledged]
      source_types: [betterstack]
  sort_by: severity

sounds:
  enabled: false

notifications:
  enabled: true
  on_new: true
  on_resolved: false
  batch_threshold: 5

actions: []

resolvers:
  - name: cluster-name
    field: label:cluster
    command: ./resolve-cluster
    stdin: value
    timeout: 500ms
    cache_ttl: 24h

ui:
  theme: system
  popup_width: 800
  popup_height: 600
  popup_position: bottom_left
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
	}
	if cfg.Sources[0].Name != "test-am" {
		t.Errorf("expected source name 'test-am', got %q", cfg.Sources[0].Name)
	}
	if cfg.Sources[0].PollInterval.Seconds() != 30 {
		t.Errorf("expected poll_interval 30s, got %v", cfg.Sources[0].PollInterval)
	}
	if cfg.Sources[0].SeverityLabel != "priority" {
		t.Errorf("expected severity_label priority, got %q", cfg.Sources[0].SeverityLabel)
	}
	if cfg.UI.PopupWidth != 800 {
		t.Errorf("expected popup_width 800, got %d", cfg.UI.PopupWidth)
	}
	if cfg.UI.PopupPosition != "bottom_left" {
		t.Errorf("expected popup_position bottom_left, got %q", cfg.UI.PopupPosition)
	}
	if cfg.Severities.Default != "info" {
		t.Fatalf("expected severity default info, got %q", cfg.Severities.Default)
	}
	if got := cfg.Severities.Levels[0].Aliases; len(got) != 2 || got[1] != "p1" {
		t.Fatalf("expected severity aliases [critical p1], got %#v", got)
	}
	if cfg.Severities.Levels[1].Color != "#00ff00" {
		t.Fatalf("expected custom info color, got %q", cfg.Severities.Levels[1].Color)
	}
	if len(cfg.Resolvers) != 1 {
		t.Fatalf("expected 1 resolver, got %d", len(cfg.Resolvers))
	}
	if cfg.Resolvers[0].Field != "label:cluster" {
		t.Fatalf("expected resolver field label:cluster, got %q", cfg.Resolvers[0].Field)
	}
	if cfg.Resolvers[0].Stdin != "value" {
		t.Fatalf("expected resolver stdin value, got %q", cfg.Resolvers[0].Stdin)
	}
	if cfg.Resolvers[0].CacheTTL != 24*time.Hour {
		t.Fatalf("expected resolver cache_ttl 24h, got %v", cfg.Resolvers[0].CacheTTL)
	}
	if got := cfg.Display.GroupByOverrides["prod"]; len(got) != 1 || got[0] != "label:namespace" {
		t.Fatalf("expected group_by_overrides for prod, got %#v", got)
	}
	if got := cfg.Display.Priority.Sources; len(got) != 1 || got[0] != "betterstack-production" {
		t.Fatalf("expected display priority source override, got %#v", got)
	}
	if got := cfg.Display.Priority.Mode; got != "after_sort" {
		t.Fatalf("expected display priority mode after_sort, got %q", got)
	}
	if got := cfg.Display.Priority.SourceTypes; len(got) != 1 || got[0] != "betterstack" {
		t.Fatalf("expected display priority source_types override, got %#v", got)
	}
	if got := cfg.Display.OverrideKeyMode(); got != "raw" {
		t.Fatalf("expected group_by_override_key_mode raw, got %q", got)
	}
	if len(cfg.Display.Badges) != 1 {
		t.Fatalf("expected 1 display badge, got %d", len(cfg.Display.Badges))
	}
	if cfg.Display.Badges[0].Label != "Ack" {
		t.Fatalf("expected display badge label Ack, got %q", cfg.Display.Badges[0].Label)
	}
}

func TestValidateDropsShellActions(t *testing.T) {
	cfg := Default()
	cfg.Actions = []ActionConfig{
		{Name: "runbook", Action: ActionDef{Type: "shell"}},
		{Name: "graph", Action: ActionDef{Type: "url", Template: "https://example.test"}},
	}

	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
	if len(cfg.Actions) != 1 || cfg.Actions[0].Name != "graph" {
		t.Fatalf("actions = %#v, want only the url action to survive", cfg.Actions)
	}
	if len(diags) != 1 || !diags[0].Dropped {
		t.Fatalf("diags = %#v, want exactly one dropped diagnostic", diags)
	}
	if !strings.Contains(diags[0].Message, "shell actions are no longer supported") {
		t.Errorf("message = %q, want shell-action migration guidance", diags[0].Message)
	}
	if !strings.Contains(diags[0].Field, `"runbook"`) {
		t.Errorf("field = %q, want the offending action name", diags[0].Field)
	}
}

func TestValidateDropsResolverProcessTemplates(t *testing.T) {
	tests := []struct {
		name     string
		resolver ResolverConfig
		want     string
	}{
		{
			name: "command",
			resolver: ResolverConfig{
				Name: "cluster-name", Field: "label:cluster", Command: "{{.Value}}", Stdin: "value",
			},
			want: "templates in command",
		},
		{
			name: "argument",
			resolver: ResolverConfig{
				Name: "cluster-name", Field: "label:cluster", Command: "./resolve-cluster", Args: []string{"{{.Value}}"}, Stdin: "value",
			},
			want: "templates in args[0]",
		},
		{
			name: "environment",
			resolver: ResolverConfig{
				Name: "cluster-name", Field: "label:cluster", Command: "./resolve-cluster", Env: map[string]string{"VALUE": "{{.Value}}"}, Stdin: "value",
			},
			want: "templates in env.VALUE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Resolvers = []ResolverConfig{
				tt.resolver,
				{Name: "keeper", Field: "label:namespace", Command: "./resolve-ns", Stdin: "value"},
			}

			var diags Diagnostics
			if err := validate(cfg, &diags); err != nil {
				t.Fatalf("validate() error = %v, want nil", err)
			}
			if len(cfg.Resolvers) != 1 || cfg.Resolvers[0].Name != "keeper" {
				t.Fatalf("resolvers = %#v, want only the valid resolver to survive", cfg.Resolvers)
			}
			if len(diags) != 1 || !diags[0].Dropped {
				t.Fatalf("diags = %#v, want exactly one dropped diagnostic", diags)
			}
			if !strings.Contains(diags[0].Message, tt.want) {
				t.Errorf("message = %q, want it to contain %q", diags[0].Message, tt.want)
			}
			if !strings.Contains(diags[0].Message, "read stdin") {
				t.Errorf("message = %q, want stdin migration guidance", diags[0].Message)
			}
		})
	}
}

func TestValidateDropsSourcesMissingRequiredFields(t *testing.T) {
	cfg := Default()
	cfg.Sources = []SourceConfig{
		{Name: "", Type: "alertmanager", URL: "http://localhost:9093"},
		{Name: "no-type", URL: "http://localhost:9093"},
		{Name: "no-url", Type: "alertmanager"},
		{Name: "good", Type: "alertmanager", URL: "http://localhost:9093"},
	}

	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].Name != "good" {
		t.Fatalf("sources = %#v, want only the valid source to survive", cfg.Sources)
	}
	if len(diags) != 3 {
		t.Fatalf("diags = %#v, want one per broken source", diags)
	}
	for _, diag := range diags {
		if !diag.Dropped {
			t.Errorf("diag %#v has Dropped = false, want true", diag)
		}
	}
}

func TestValidateDropsUnsupportedDuplicateAndNegativeIntervalSources(t *testing.T) {
	cfg := Default()
	cfg.Sources = []SourceConfig{
		{Name: "unsupported", Type: "mystery", URL: "https://alerts.example.test"},
		{Name: "negative", Type: "alertmanager", URL: "https://alerts.example.test", PollInterval: -time.Second},
		{Name: "duplicate", Type: "alertmanager", URL: "https://one.example.test"},
		{Name: "duplicate", Type: "grafana", URL: "https://two.example.test"},
		{Name: "normalized", Type: " GRAFANA ", URL: "https://grafana.example.test"},
	}

	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
	if len(cfg.Sources) != 2 || cfg.Sources[0].Name != "duplicate" || cfg.Sources[1].Name != "normalized" {
		t.Fatalf("sources = %#v, want first duplicate and normalized source", cfg.Sources)
	}
	if cfg.Sources[1].Type != "grafana" {
		t.Errorf("normalized source type = %q, want grafana", cfg.Sources[1].Type)
	}
	if len(diags) != 3 {
		t.Fatalf("diags = %#v, want unsupported, negative interval, and duplicate diagnostics", diags)
	}
	for _, diag := range diags {
		if !diag.Dropped {
			t.Errorf("diag %#v has Dropped = false, want true", diag)
		}
	}
}

func TestValidateDropsBrokenHideRules(t *testing.T) {
	cfg := Default()
	cfg.Hide = []HideRule{
		{Name: "no-matchers"},
		{Name: "bad-matcher", Matchers: []string{"no-operator-here"}},
		{Name: "bad-age", Matchers: []string{"alertname=Watchdog"}, MinAge: "not-a-duration"},
		{Name: "negative-age", Matchers: []string{"alertname=Watchdog"}, MinAge: "-5m"},
		{Name: "good", Matchers: []string{"alertname=Watchdog"}, MinAge: "30m"},
	}

	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
	if len(cfg.Hide) != 1 || cfg.Hide[0].Name != "good" {
		t.Fatalf("hide = %#v, want only the valid rule to survive", cfg.Hide)
	}
	if cfg.Hide[0].ParsedMinAge != 30*time.Minute {
		t.Errorf("ParsedMinAge = %v, want 30m", cfg.Hide[0].ParsedMinAge)
	}
	if len(diags) != 4 {
		t.Fatalf("diags = %#v, want one per broken rule", diags)
	}
}

func TestValidateSubstitutesDefaultsForBrokenUIFields(t *testing.T) {
	cfg := Default()
	cfg.UI.PopupPosition = "middle"
	cfg.UI.Scale.Mode = "sideways"
	cfg.UI.Scale.Factor = 9

	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
	if cfg.UI.PopupPosition != "top_right" {
		t.Errorf("PopupPosition = %q, want top_right", cfg.UI.PopupPosition)
	}
	if cfg.UI.Scale.Mode != "fonts" {
		t.Errorf("Scale.Mode = %q, want fonts", cfg.UI.Scale.Mode)
	}
	if cfg.UI.Scale.Factor != 2.0 {
		t.Errorf("Scale.Factor = %v, want it clamped to 2.0", cfg.UI.Scale.Factor)
	}
	if len(diags) != 3 {
		t.Fatalf("diags = %#v, want one per substituted field", diags)
	}
	for _, diag := range diags {
		if diag.Dropped {
			t.Errorf("diag %#v has Dropped = true, want false for a substitution", diag)
		}
	}
}

func TestValidateSubstitutesDefaultSeveritiesWhenInvalid(t *testing.T) {
	cfg := Default()
	cfg.Severities = SeverityConfig{
		Default: "info",
		Levels: []SeverityLevel{
			{Name: "critical", Aliases: []string{"p1"}},
			{Name: "warning", Aliases: []string{"p1"}},
		},
	}

	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
	if len(diags) != 1 || diags[0].Dropped || diags[0].Field != "severities" {
		t.Fatalf("diags = %#v, want one substitution diagnostic for severities", diags)
	}
	if len(cfg.Severities.Levels) == 0 {
		t.Fatal("severity levels are empty, want the built-in defaults")
	}
}

func TestValidateResolverStdin(t *testing.T) {
	t.Run("required", func(t *testing.T) {
		cfg := Default()
		cfg.Resolvers = []ResolverConfig{{Name: "cluster-name", Field: "label:cluster", Command: "./resolve-cluster"}}

		var diags Diagnostics
		if err := validate(cfg, &diags); err != nil {
			t.Fatalf("validate() error = %v, want nil", err)
		}
		if len(cfg.Resolvers) != 0 {
			t.Fatalf("resolvers = %#v, want the resolver dropped", cfg.Resolvers)
		}
		if len(diags) != 1 || !strings.Contains(diags[0].Message, "stdin is required") {
			t.Fatalf("diags = %#v, want a required-stdin diagnostic", diags)
		}
	})

	t.Run("normalizes json", func(t *testing.T) {
		cfg := Default()
		cfg.Resolvers = []ResolverConfig{{
			Field: " label:cluster ", Command: " ./resolve-cluster ", Args: []string{"--lookup"},
			Env: map[string]string{"MODE": "fixed"}, Stdin: " JSON ",
		}}

		var diags Diagnostics
		if err := validate(cfg, &diags); err != nil {
			t.Fatalf("validate() error: %v", err)
		}
		if len(diags) != 0 {
			t.Fatalf("diags = %#v, want none", diags)
		}
		resolver := cfg.Resolvers[0]
		if resolver.Field != "label:cluster" || resolver.Command != "./resolve-cluster" || resolver.Stdin != "json" {
			t.Fatalf("resolver was not normalized: %#v", resolver)
		}
	})
}

func TestLoadDropsInvalidSourceKeepsValidNeighbor(t *testing.T) {
	yamlBody := `
sources:
  - name: broken
    type: alertmanager
  - name: good
    type: alertmanager
    url: http://localhost:9093
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, diags, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].Name != "good" {
		t.Fatalf("sources = %#v, want only the valid neighbour to survive", cfg.Sources)
	}
	if len(diags) != 1 || !diags[0].Dropped {
		t.Fatalf("diags = %#v, want exactly one dropped diagnostic", diags)
	}
}

func TestLoadConfigSourceTimeout(t *testing.T) {
	yaml := `
sources:
  - name: with-timeout
    type: alertmanager
    url: http://localhost:9093
    timeout: 5s
  - name: without-timeout
    type: alertmanager
    url: http://localhost:9094
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.Sources[0].Timeout; got != 5*time.Second {
		t.Fatalf("expected explicit timeout 5s, got %v", got)
	}
	if got := cfg.Sources[1].Timeout; got != DefaultSourceTimeout {
		t.Fatalf("expected default timeout %v, got %v", DefaultSourceTimeout, got)
	}
}

func TestLoadConfigFiltersDisabledSources(t *testing.T) {
	yaml := `
sources:
  - name: enabled-by-default
    type: alertmanager
    url: http://localhost:9093
  - name: explicitly-enabled
    type: alertmanager
    enabled: true
    url: http://localhost:9094
  - name: disabled
    type: alertmanager
    enabled: false
    url: http://localhost:9095
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := len(cfg.Sources); got != 2 {
		t.Fatalf("expected 2 enabled sources, got %d", got)
	}
	if cfg.Sources[0].Name != "enabled-by-default" {
		t.Fatalf("expected omitted enabled source to stay enabled, got %q", cfg.Sources[0].Name)
	}
	if cfg.Sources[1].Name != "explicitly-enabled" {
		t.Fatalf("expected explicitly enabled source, got %q", cfg.Sources[1].Name)
	}
}

func TestLoadConfigDefaultsSeverityLabel(t *testing.T) {
	yaml := `
sources:
  - name: test
    type: grafana
    url: http://localhost:3000
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Sources[0].SeverityLabel != "severity" {
		t.Fatalf("expected default severity_label severity, got %q", cfg.Sources[0].SeverityLabel)
	}
}

func TestLoadConfigBetterStackDefaultsURL(t *testing.T) {
	yaml := `
sources:
  - name: better
    type: betterstack
    auth:
      type: bearer
      token: secret
    betterstack:
      on_call_schedule: default
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Sources[0].URL != "https://uptime.betterstack.com" {
		t.Fatalf("expected Better Stack default URL, got %q", cfg.Sources[0].URL)
	}
	if cfg.Sources[0].BetterStack.OnCallSchedule != "default" {
		t.Fatalf("expected on_call_schedule default, got %q", cfg.Sources[0].BetterStack.OnCallSchedule)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	os.Setenv("FOGHORN_TEST_USER", "testuser")
	os.Setenv("FOGHORN_TEST_PASS", "testpass")
	defer os.Unsetenv("FOGHORN_TEST_USER")
	defer os.Unsetenv("FOGHORN_TEST_PASS")

	yaml := `
sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
    auth:
      type: basic
      username: ${FOGHORN_TEST_USER}
      password: ${FOGHORN_TEST_PASS}
    poll_interval: 30s
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Sources[0].Auth.Username != "testuser" {
		t.Errorf("expected expanded username 'testuser', got %q", cfg.Sources[0].Auth.Username)
	}
	if cfg.Sources[0].Auth.Password != "testpass" {
		t.Errorf("expected expanded password 'testpass', got %q", cfg.Sources[0].Auth.Password)
	}
}

func TestLoadConfigInvalidPopupPositionReportsExpandedValue(t *testing.T) {
	t.Setenv("FOGHORN_TEST_POPUP_POSITION", " Sideways ")

	yaml := `
sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  popup_position: ${FOGHORN_TEST_POPUP_POSITION}
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, diags, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.UI.PopupPosition != "top_right" {
		t.Fatalf("PopupPosition = %q, want the top_right default", cfg.UI.PopupPosition)
	}
	if len(diags) != 1 {
		t.Fatalf("diags = %#v, want exactly one", diags)
	}
	if diags[0].Field != "ui.popup_position" {
		t.Errorf("field = %q, want ui.popup_position", diags[0].Field)
	}
	if diags[0].Dropped {
		t.Errorf("Dropped = true, want false for a substituted default")
	}
	// The raw value keeps the casing the env var supplied (YAML strips the
	// surrounding spaces), so the diagnostic must show both forms — that is
	// what makes an env-var typo diagnosable.
	for _, want := range []string{`"Sideways"`, `normalized: "sideways"`} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("message = %q, want it to contain %q", diags[0].Message, want)
		}
	}
}

func TestDefaultEnablesNewAlertNotifications(t *testing.T) {
	cfg := Default()

	if !cfg.Notifications.Enabled {
		t.Fatal("expected notifications to be enabled by default")
	}
	if !cfg.Notifications.OnNew {
		t.Fatal("expected new alert notifications to be enabled by default")
	}
	if cfg.Notifications.OnResolved {
		t.Fatal("expected resolved alert notifications to remain disabled by default")
	}
	if cfg.Notifications.BatchThreshold != 5 {
		t.Fatalf("expected default batch threshold 5, got %d", cfg.Notifications.BatchThreshold)
	}
}

func writeAndLoad(t *testing.T, yamlBody string) *Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

const minimalSource = `
sources:
  - name: test-am
    type: alertmanager
    url: http://localhost:9093
`

func TestLoadOIDCPersistTokensExplicitFalse(t *testing.T) {
	cfg := writeAndLoad(t, `
sources:
  - name: oidc-am
    type: alertmanager
    url: https://alertmanager.example.test
    auth:
      type: oidc
      flow: device
      issuer_url: https://login.example.test
      client_id: foghorn
      persist_tokens: false
`)
	persist := cfg.Sources[0].Auth.PersistTokens
	if persist == nil || *persist {
		t.Fatalf("persist_tokens = %v, want explicit false", persist)
	}
}

func TestSilenceEditorDefaultsWhenAbsent(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource)
	se := cfg.UI.SilenceEditor
	if se.AlwaysVisibleMatchers == nil {
		t.Fatal("always_visible_matchers should be resolved to a non-nil default")
	}
	got := *se.AlwaysVisibleMatchers
	want := []string{"alertname", "cluster", "severity", "pod"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("default whitelist = %v, want %v", got, want)
	}
	if se.CollapseMatchers == nil || !(*se.CollapseMatchers) {
		t.Fatalf("collapse_matchers default = %v, want true", se.CollapseMatchers)
	}
}

func TestPopupFollowCursorDefaultsTrue(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource)
	if cfg.UI.PopupFollowCursor == nil {
		t.Fatal("popup_follow_cursor should be resolved to a non-nil default")
	}
	if !*cfg.UI.PopupFollowCursor {
		t.Fatalf("popup_follow_cursor default = %v, want true", *cfg.UI.PopupFollowCursor)
	}
}

func TestPopupFollowCursorExplicitFalse(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource+`
ui:
  popup_follow_cursor: false
`)
	if cfg.UI.PopupFollowCursor == nil || *cfg.UI.PopupFollowCursor {
		t.Fatalf("popup_follow_cursor = %v, want false", cfg.UI.PopupFollowCursor)
	}
}

func TestAutoPositionDefaultsTrue(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource)
	if cfg.UI.AutoPosition == nil {
		t.Fatal("auto_position should be resolved to a non-nil default")
	}
	if !*cfg.UI.AutoPosition {
		t.Fatalf("auto_position default = %v, want true", *cfg.UI.AutoPosition)
	}
}

func TestAutoPositionExplicitFalse(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource+`
ui:
  auto_position: false
`)
	if cfg.UI.AutoPosition == nil || *cfg.UI.AutoPosition {
		t.Fatalf("auto_position = %v, want false", cfg.UI.AutoPosition)
	}
}

func TestSilenceEditorExplicitEmptyWhitelist(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource+`
ui:
  silence_editor:
    always_visible_matchers: []
`)
	se := cfg.UI.SilenceEditor
	if se.AlwaysVisibleMatchers == nil {
		t.Fatal("explicit empty whitelist should stay non-nil (empty), not become default")
	}
	if len(*se.AlwaysVisibleMatchers) != 0 {
		t.Fatalf("explicit empty whitelist = %v, want empty", *se.AlwaysVisibleMatchers)
	}
}

func TestSilenceEditorCollapseDisabled(t *testing.T) {
	cfg := writeAndLoad(t, minimalSource+`
ui:
  silence_editor:
    collapse_matchers: false
`)
	se := cfg.UI.SilenceEditor
	if se.CollapseMatchers == nil || *se.CollapseMatchers {
		t.Fatalf("collapse_matchers = %v, want false", se.CollapseMatchers)
	}
}

func TestDefaultPopulatesSilenceEditor(t *testing.T) {
	cfg := Default()
	if cfg.UI.SilenceEditor.AlwaysVisibleMatchers == nil || cfg.UI.SilenceEditor.CollapseMatchers == nil {
		t.Fatal("Default() must populate SilenceEditor pointer fields")
	}
}

func TestLoadConfigSubstitutesDefaultsForDuplicateSeverityAliases(t *testing.T) {
	yaml := `
sources:
  - name: test
    type: alertmanager
    url: http://localhost:9093
severities:
  levels:
    - name: critical
      aliases: [critical, sev1]
    - name: warning
      aliases: [warning, sev1]
display:
  visible_labels: []
  visible_annotations: []
  group_by: []
  sort_by: severity
sounds:
  enabled: false
notifications:
  enabled: false
  on_new: false
  on_resolved: false
  batch_threshold: 5
actions: []
ui:
  theme: system
  popup_width: 800
  popup_height: 600
  show_resolved: false
  show_silenced: true
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, diags, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(diags) != 1 || diags[0].Field != "severities" || diags[0].Dropped {
		t.Fatalf("diags = %#v, want one substitution diagnostic for severities", diags)
	}
	for _, want := range []string{`alias "sev1"`, "already assigned"} {
		if !strings.Contains(diags[0].Message, want) {
			t.Errorf("message = %q, want it to contain %q", diags[0].Message, want)
		}
	}
	if len(cfg.Severities.Levels) == 0 {
		t.Fatal("severity levels are empty, want the built-in defaults")
	}
}

func TestLoadConfigVisibleEntriesMixed(t *testing.T) {
	yaml := `
sources:
  - name: test-am
    type: alertmanager
    url: http://localhost:9093

display:
  visible_annotations:
    - source: field:hiddenBy
      order: -5
      label: Hidden By
      style: muted
    - summary
    - link
    - source: description
      order: 5
      label: Description
      style: [pull, danger]
  visible_labels:
    - cluster:raw
    - source: namespace
      style: muted
  group_by: [cluster]
  sort_by: severity

ui:
  popup_position: top_right
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	gotAnnotationSources := make([]string, len(cfg.Display.VisibleAnnotations))
	for i, e := range cfg.Display.VisibleAnnotations {
		gotAnnotationSources[i] = e.Source
	}
	wantAnnotationSources := []string{"field:hiddenBy", "summary", "link", "description"}
	if len(gotAnnotationSources) != len(wantAnnotationSources) {
		t.Fatalf("annotation count: got %d (%v), want %d (%v)", len(gotAnnotationSources), gotAnnotationSources, len(wantAnnotationSources), wantAnnotationSources)
	}
	for i := range wantAnnotationSources {
		if gotAnnotationSources[i] != wantAnnotationSources[i] {
			t.Fatalf("annotation order = %v, want %v", gotAnnotationSources, wantAnnotationSources)
		}
	}

	// The "description" entry should carry the styles set in YAML.
	descEntry := cfg.Display.VisibleAnnotations[3]
	if descEntry.Label != "Description" {
		t.Errorf("description label = %q, want %q", descEntry.Label, "Description")
	}
	if len(descEntry.Style) != 2 || descEntry.Style[0] != StylePull || descEntry.Style[1] != StyleDanger {
		t.Errorf("description style = %#v, want [pull danger]", descEntry.Style)
	}

	// Labels: cluster:raw is bare (Order 0, list pos 0); namespace is mapping (Order 0, list pos 1).
	if len(cfg.Display.VisibleLabels) != 2 {
		t.Fatalf("expected 2 visible labels, got %d", len(cfg.Display.VisibleLabels))
	}
	if cfg.Display.VisibleLabels[0].Source != "cluster:raw" {
		t.Errorf("labels[0].Source = %q, want %q", cfg.Display.VisibleLabels[0].Source, "cluster:raw")
	}
	if cfg.Display.VisibleLabels[1].Source != "namespace" || len(cfg.Display.VisibleLabels[1].Style) != 1 || cfg.Display.VisibleLabels[1].Style[0] != StyleMuted {
		t.Errorf("labels[1] = %#v", cfg.Display.VisibleLabels[1])
	}
}

func TestLoadConfigVisibleEntriesUnknownStyle(t *testing.T) {
	yaml := `
sources:
  - name: test-am
    type: alertmanager
    url: http://localhost:9093

display:
  visible_annotations:
    - summary
    - source: description
      style: ominous
  visible_labels: []
  group_by: [cluster]
  sort_by: severity

ui:
  popup_position: top_right
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, diags, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(cfg.Display.VisibleAnnotations) != 1 || cfg.Display.VisibleAnnotations[0].Source != "summary" {
		t.Fatalf("visible_annotations = %#v, want only the valid entry to survive", cfg.Display.VisibleAnnotations)
	}
	if len(diags) != 1 {
		t.Fatalf("diags = %#v, want exactly one", diags)
	}
	if !diags[0].Dropped {
		t.Errorf("Dropped = false, want true for a dropped entry")
	}
	if !strings.Contains(diags[0].Message, "unknown style") {
		t.Errorf("message = %q, want it to contain %q", diags[0].Message, "unknown style")
	}
}

func TestLoadConfigUIScaleDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(minimalConfig), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.UI.Scale.Factor != 1.0 {
		t.Fatalf("expected default scale factor 1.0, got %v", cfg.UI.Scale.Factor)
	}
	if cfg.UI.Scale.Mode != "fonts" {
		t.Fatalf("expected default scale mode fonts, got %q", cfg.UI.Scale.Mode)
	}
	if !cfg.UI.Scale.ApplyToPopup {
		t.Fatal("expected default scale apply_to_popup true")
	}
}

func TestLoadConfigUIScalePartialDefaults(t *testing.T) {
	yaml := minimalConfigWithUIScale("    factor: 1.25\n")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.UI.Scale.Factor != 1.25 {
		t.Fatalf("expected scale factor 1.25, got %v", cfg.UI.Scale.Factor)
	}
	if cfg.UI.Scale.Mode != "fonts" {
		t.Fatalf("expected default scale mode fonts, got %q", cfg.UI.Scale.Mode)
	}
	if !cfg.UI.Scale.ApplyToPopup {
		t.Fatal("expected default scale apply_to_popup true")
	}
}

func TestLoadConfigUIScaleClampsFactor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{name: "lower", in: "0.5", want: 0.75},
		{name: "upper", in: "5.0", want: 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := minimalConfigWithUIScale("    factor: " + tt.in + "\n")
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
				t.Fatal(err)
			}

			cfg, diags, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.UI.Scale.Factor != tt.want {
				t.Fatalf("expected scale factor %v, got %v", tt.want, cfg.UI.Scale.Factor)
			}
			found := false
			for _, diag := range diags {
				if diag.Field == "ui.scale.factor" {
					found = true
					if diag.Dropped {
						t.Errorf("diag %#v has Dropped = true, want false for a substitution", diag)
					}
				}
			}
			if !found {
				t.Errorf("diags = %#v, want a ui.scale.factor diagnostic", diags)
			}
		})
	}
}

func TestLoadConfigUIScaleInvalidMode(t *testing.T) {
	yaml := minimalConfigWithUIScale("    mode: huge\n")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, diags, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.UI.Scale.Mode != "fonts" {
		t.Fatalf("Scale.Mode = %q, want fonts", cfg.UI.Scale.Mode)
	}
	if len(diags) != 1 || diags[0].Field != "ui.scale.mode" || diags[0].Dropped {
		t.Fatalf("diags = %#v, want one substitution diagnostic for ui.scale.mode", diags)
	}
}

func minimalConfigWithUIScale(scaleBody string) string {
	return strings.Replace(minimalConfig, "  show_silenced: true\n", "  show_silenced: true\n  scale:\n"+scaleBody, 1)
}

// A source URL over plain HTTP is already warned about; the OIDC endpoints
// deserve the same treatment, because a cleartext issuer lets an on-path
// attacker pick the real endpoints and cleartext device/token endpoints carry
// the client_id and client_secret in an HTTP Basic header.
func TestLoadConfigWarnsAboutCleartextOIDCEndpoints(t *testing.T) {
	yaml := `
sources:
  - name: sso
    type: alertmanager
    url: https://alerts.example.test
    auth:
      type: oidc
      flow: device
      issuer_url: http://login.example.test
      client_id: foghorn
  - name: manual
    type: alertmanager
    url: https://alerts.example.test
    auth:
      type: oidc
      flow: device
      device_authorization_url: http://sso.example.test/device
      token_url: http://sso.example.test/token
      client_id: foghorn
  - name: local
    type: alertmanager
    url: https://alerts.example.test
    auth:
      type: oidc
      flow: device
      issuer_url: http://127.0.0.1:8080
      client_id: foghorn
  - name: secure
    type: alertmanager
    url: https://alerts.example.test
    auth:
      type: oidc
      flow: device
      issuer_url: https://login.example.test
      client_id: foghorn
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	var logs strings.Builder
	original := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(original)

	if _, _, err := Load(path); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	output := logs.String()
	for _, want := range []string{"issuer_url", "device_authorization_url", "token_url"} {
		if !strings.Contains(output, want) {
			t.Errorf("no cleartext warning for auth.%s in:\n%s", want, output)
		}
	}
	if strings.Contains(output, "127.0.0.1") {
		t.Errorf("loopback issuer must not warn:\n%s", output)
	}
	if strings.Contains(output, `source "secure"`) {
		t.Errorf("https issuer must not warn:\n%s", output)
	}
}
