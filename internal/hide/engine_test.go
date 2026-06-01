package hide

import (
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func newEngine(t *testing.T, rules []config.HideRule) *Engine {
	t.Helper()
	for i := range rules {
		if rules[i].MinAge == "" || rules[i].ParsedMinAge != 0 {
			continue
		}
		// Test inputs use MinAge as a string; mimic config.validate by
		// parsing here so callers don't have to pre-fill ParsedMinAge.
		d, err := time.ParseDuration(rules[i].MinAge)
		if err != nil {
			t.Fatalf("test setup: parsing MinAge %q: %v", rules[i].MinAge, err)
		}
		rules[i].ParsedMinAge = d
	}
	e, errs := New(rules)
	if len(errs) > 0 {
		t.Fatalf("unexpected compile errors: %v", errs)
	}
	return e
}

func fixedNow(ts time.Time) func() time.Time {
	return func() time.Time { return ts }
}

func TestMatcherEquality(t *testing.T) {
	e := newEngine(t, []config.HideRule{{
		Name:     "deadman",
		Matchers: []string{"alertname=PrometheusWatchdog"},
	}})

	got := e.Apply([]model.Alert{
		{ID: "1", Name: "PrometheusWatchdog"},
		{ID: "2", Name: "RealAlert"},
	})

	if want := []string{"deadman"}; !equalSlices(got[0].HiddenBy, want) {
		t.Errorf("alert 1 HiddenBy = %v, want %v", got[0].HiddenBy, want)
	}
	if got[1].HiddenBy != nil {
		t.Errorf("alert 2 HiddenBy = %v, want nil", got[1].HiddenBy)
	}
}

func TestMatcherInequality(t *testing.T) {
	e := newEngine(t, []config.HideRule{{
		Name:     "non-prod",
		Matchers: []string{"environment!=production"},
	}})

	got := e.Apply([]model.Alert{
		{ID: "1", Labels: map[string]string{"environment": "staging"}},
		{ID: "2", Labels: map[string]string{"environment": "production"}},
		{ID: "3"}, // no environment label at all
	})

	if got[0].HiddenBy == nil {
		t.Errorf("alert 1 should be hidden (env=staging != production)")
	}
	if got[1].HiddenBy != nil {
		t.Errorf("alert 2 should not be hidden (env=production)")
	}
	if got[2].HiddenBy == nil {
		t.Errorf("alert 3 should be hidden (missing label != production)")
	}
}

func TestMatcherRegex(t *testing.T) {
	e := newEngine(t, []config.HideRule{{
		Name:     "watchdogs",
		Matchers: []string{`alertname=~"^Watchdog.*"`},
	}})

	got := e.Apply([]model.Alert{
		{ID: "1", Name: "WatchdogFoo"},
		{ID: "2", Name: "OtherAlert"},
	})

	if got[0].HiddenBy == nil {
		t.Errorf("alert 1 should be hidden")
	}
	if got[1].HiddenBy != nil {
		t.Errorf("alert 2 should not be hidden")
	}
}

func TestMatcherRawVsResolved(t *testing.T) {
	e := newEngine(t, []config.HideRule{{
		Name:     "raw-only",
		Matchers: []string{"cluster:raw=cs-2a"},
	}})

	// alert has raw cluster=cs-2a, resolved=Cluster 2A
	got := e.Apply([]model.Alert{{
		ID:             "1",
		Labels:         map[string]string{"cluster": "cs-2a"},
		ResolvedLabels: map[string]string{"cluster": "Cluster 2A"},
	}})

	if got[0].HiddenBy == nil {
		t.Errorf("cluster:raw=cs-2a should match raw value")
	}

	e2 := newEngine(t, []config.HideRule{{
		Name:     "resolved-only",
		Matchers: []string{`cluster:resolved=cs-2a`},
	}})
	got2 := e2.Apply([]model.Alert{{
		ID:             "1",
		Labels:         map[string]string{"cluster": "cs-2a"},
		ResolvedLabels: map[string]string{"cluster": "Cluster 2A"},
	}})
	if got2[0].HiddenBy != nil {
		t.Errorf("cluster:resolved=cs-2a should NOT match resolved value 'Cluster 2A'")
	}
}

func TestSourceScoping(t *testing.T) {
	e := newEngine(t, []config.HideRule{{
		Name:     "scoped",
		Matchers: []string{"severity=none"},
		Sources:  []string{"central1"},
	}})

	got := e.Apply([]model.Alert{
		{ID: "1", Source: "central1", Severity: "none"},
		{ID: "2", Source: "other", Severity: "none"},
	})

	if got[0].HiddenBy == nil {
		t.Errorf("alert from central1 should be hidden")
	}
	if got[1].HiddenBy != nil {
		t.Errorf("alert from other source should not be hidden")
	}
}

func TestMinAge(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	rules := []config.HideRule{{
		Name:     "stable",
		Matchers: []string{"alertname=PrometheusWatchdog"},
		MinAge:   "24h",
	}}
	e := newEngine(t, rules)
	e.now = fixedNow(now)

	got := e.Apply([]model.Alert{
		{ID: "fresh", Name: "PrometheusWatchdog", StartsAt: now.Add(-1 * time.Hour)},
		{ID: "old", Name: "PrometheusWatchdog", StartsAt: now.Add(-48 * time.Hour)},
	})

	if got[0].HiddenBy != nil {
		t.Errorf("1h-old alert should not yet be hidden by min_age=24h")
	}
	if got[1].HiddenBy == nil {
		t.Errorf("48h-old alert should be hidden by min_age=24h")
	}
}

func TestANDAcrossMatchers(t *testing.T) {
	e := newEngine(t, []config.HideRule{{
		Name:     "deadman",
		Matchers: []string{"alertname=PrometheusWatchdog", "severity=none"},
	}})

	got := e.Apply([]model.Alert{
		{ID: "match", Name: "PrometheusWatchdog", Severity: "none"},
		{ID: "partial1", Name: "PrometheusWatchdog", Severity: "warning"},
		{ID: "partial2", Name: "OtherAlert", Severity: "none"},
	})

	if got[0].HiddenBy == nil {
		t.Errorf("alert matching all matchers should be hidden")
	}
	if got[1].HiddenBy != nil || got[2].HiddenBy != nil {
		t.Errorf("alerts matching only one matcher should not be hidden")
	}
}

func TestInvalidMatcherReportsError(t *testing.T) {
	_, errs := New([]config.HideRule{{
		Name:     "bad",
		Matchers: []string{"no-operator-here"},
	}})
	if len(errs) == 0 {
		t.Fatalf("expected compile error for matcher without operator")
	}
}

func TestParseMatcherEdgeCases(t *testing.T) {
	cases := []struct {
		raw       string
		wantField string
		wantOp    string
		wantValue string
	}{
		{"alertname=Foo", "alertname", "=", "Foo"},
		{`alertname="Foo Bar"`, "alertname", "=", "Foo Bar"},
		{"severity!=warning", "severity", "!=", "warning"},
		{`alertname=~^Watch.*`, "alertname", "=~", "^Watch.*"},
		{"label:cluster:raw=cs-2a", "label:cluster:raw", "=", "cs-2a"},
		// Value contains '=' — only the leftmost operator counts.
		{"annotation:summary=a=b", "annotation:summary", "=", "a=b"},
	}
	for _, c := range cases {
		m, err := parseMatcher(c.raw)
		if err != nil {
			t.Errorf("parseMatcher(%q) error: %v", c.raw, err)
			continue
		}
		if m.field != c.wantField || m.op != c.wantOp || m.value != c.wantValue {
			t.Errorf("parseMatcher(%q) = (%q, %q, %q), want (%q, %q, %q)",
				c.raw, m.field, m.op, m.value, c.wantField, c.wantOp, c.wantValue)
		}
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
