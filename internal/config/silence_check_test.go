package config

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func validateOne(t *testing.T, src SourceConfig) (SourceConfig, Diagnostics) {
	t.Helper()
	cfg := Default()
	cfg.Sources = []SourceConfig{src}
	var diags Diagnostics
	if err := validate(cfg, &diags); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("source dropped: %#v", diags)
	}
	return cfg.Sources[0], diags
}

func boolPtr(v bool) *bool                  { return &v }
func durPtr(v time.Duration) *time.Duration { return &v }
func grafanaSrc(sc SilenceCheckConfig) SourceConfig {
	return SourceConfig{Name: "g", Type: "grafana", URL: "http://localhost:3000", SilenceCheck: sc}
}

func TestSilenceCheckDefaults(t *testing.T) {
	src, diags := validateOne(t, grafanaSrc(SilenceCheckConfig{}))
	sc := src.SilenceCheck
	if sc.Granularity != "folder" || sc.OnStart == nil || !*sc.OnStart || sc.Interval == nil || *sc.Interval != 15*time.Minute || sc.OnDenied == nil || !*sc.OnDenied {
		t.Fatalf("unexpected defaults %+v", sc)
	}
	if len(diags) != 0 {
		t.Fatalf("unexpected diags %#v", diags)
	}
}

func TestSilenceCheckUnknownGranularity(t *testing.T) {
	src, diags := validateOne(t, grafanaSrc(SilenceCheckConfig{Granularity: "everything"}))
	if src.SilenceCheck.Granularity != "folder" {
		t.Fatalf("granularity %q", src.SilenceCheck.Granularity)
	}
	if len(diags) != 1 || diags[0].Dropped || diags[0].Field != `sources[0] "g" silence_check.granularity` {
		t.Fatalf("diags %#v", diags)
	}
}

func TestSilenceCheckOnNonGrafana(t *testing.T) {
	_, diags := validateOne(t, SourceConfig{Name: "am", Type: "alertmanager", URL: "http://localhost:9093",
		SilenceCheck: SilenceCheckConfig{Granularity: "source"}})
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "only applies to grafana sources; ignored") {
		t.Fatalf("diags %#v", diags)
	}
}

func TestSilenceCheckNothingRuns(t *testing.T) {
	src, diags := validateOne(t, grafanaSrc(SilenceCheckConfig{Granularity: "source", OnStart: boolPtr(false), Interval: durPtr(0)}))
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "only refused writes will grey out controls") {
		t.Fatalf("diags %#v", diags)
	}
	if *src.SilenceCheck.Interval != 0 || *src.SilenceCheck.OnStart {
		t.Fatalf("values changed %+v", src.SilenceCheck)
	}
}

func TestSilenceCheckNegativeInterval(t *testing.T) {
	src, diags := validateOne(t, grafanaSrc(SilenceCheckConfig{Interval: durPtr(-time.Minute)}))
	if *src.SilenceCheck.Interval != 15*time.Minute || len(diags) != 1 {
		t.Fatalf("interval %v diags %#v", *src.SilenceCheck.Interval, diags)
	}
}

func TestSilenceCheckYAML(t *testing.T) {
	var sc SilenceCheckConfig
	if err := yamlUnmarshalForTest("granularity: source\non_start: false\ninterval: 5m\non_denied: false\n", &sc); err != nil {
		t.Fatal(err)
	}
	if sc.Granularity != "source" || *sc.OnStart || *sc.Interval != 5*time.Minute || *sc.OnDenied {
		t.Fatalf("parsed %+v", sc)
	}
}

func yamlUnmarshalForTest(s string, out any) error { return yaml.Unmarshal([]byte(s), out) }
