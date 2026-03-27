package resolve

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func TestResolveAlertLabelCommand(t *testing.T) {
	engine := New([]config.ResolverConfig{
		{
			Name:    "cluster",
			Field:   "label:cluster",
			Command: "sh",
			Args:    []string{"-c", "printf '%s' \"$1-resolved\"", "--", "{{.Value}}"},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Name:   "TargetDown",
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	resolved := engine.ResolveAlert(alert)
	if got := resolved.ResolvedLabels["cluster"]; got != "saas-cs-0b-resolved" {
		t.Fatalf("expected resolved cluster label, got %q", got)
	}
	if got := resolved.Labels["cluster"]; got != "saas-cs-0b" {
		t.Fatalf("expected raw cluster label to stay untouched, got %q", got)
	}
}

func TestResolveAlertFieldCommand(t *testing.T) {
	engine := New([]config.ResolverConfig{
		{
			Name:    "source",
			Field:   "field:source",
			Command: "sh",
			Args:    []string{"-c", "printf '%s/%s' \"$1\" \"$2\"", "--", "{{.Value}}", "{{.Labels.cluster}}"},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Source: "prod-am",
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	resolved := engine.ResolveAlert(alert)
	if got := resolved.ResolvedFields["source"]; got != "prod-am/saas-cs-0b" {
		t.Fatalf("expected resolved source field, got %q", got)
	}
}

func TestResolveAlertUsesCache(t *testing.T) {
	engine := New([]config.ResolverConfig{
		{
			Name:    "cluster",
			Field:   "label:cluster",
			Command: "sh",
			Args: []string{
				"-c",
				`count=$(cat "$1" 2>/dev/null || echo 0); count=$((count+1)); printf '%s' "$count" > "$1"; printf 'customer-%s' "$count"`,
				"--",
				filepath.Join(t.TempDir(), "resolver-count"),
			},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	first := engine.ResolveAlert(alert)
	second := engine.ResolveAlert(alert)

	if got := first.ResolvedLabels["cluster"]; got != "customer-1" {
		t.Fatalf("expected first resolved value customer-1, got %q", got)
	}
	if got := second.ResolvedLabels["cluster"]; got != "customer-1" {
		t.Fatalf("expected cached resolved value customer-1, got %q", got)
	}
}

func TestResolveAlertCacheTTLExpires(t *testing.T) {
	originalNow := timeNow
	t.Cleanup(func() {
		timeNow = originalNow
	})

	current := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return current }

	counterDir := t.TempDir()
	counterFile := filepath.Join(counterDir, "resolver-count")
	engine := New([]config.ResolverConfig{
		{
			Name:     "cluster",
			Field:    "label:cluster",
			Command:  "sh",
			Args:     []string{"-c", `count=$(cat "$1" 2>/dev/null || echo 0); count=$((count+1)); printf '%s' "$count" > "$1"; printf 'customer-%s' "$count"`, "--", counterFile},
			Timeout:  time.Second,
			CacheTTL: time.Minute,
		},
	})

	alert := model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	first := engine.ResolveAlert(alert)
	current = current.Add(30 * time.Second)
	second := engine.ResolveAlert(alert)
	current = current.Add(61 * time.Second)
	third := engine.ResolveAlert(alert)

	if got := first.ResolvedLabels["cluster"]; got != "customer-1" {
		t.Fatalf("expected first resolved value customer-1, got %q", got)
	}
	if got := second.ResolvedLabels["cluster"]; got != "customer-1" {
		t.Fatalf("expected cached value before ttl expiry, got %q", got)
	}
	if got := third.ResolvedLabels["cluster"]; got != "customer-2" {
		t.Fatalf("expected refreshed value after ttl expiry, got %q", got)
	}

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("reading counter file: %v", err)
	}
	if string(data) != "2" {
		t.Fatalf("expected resolver command to run twice, got %q", string(data))
	}
}

func TestResolveAlertCachesFailuresBriefly(t *testing.T) {
	originalNow := timeNow
	t.Cleanup(func() {
		timeNow = originalNow
	})

	current := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return current }

	counterDir := t.TempDir()
	counterFile := filepath.Join(counterDir, "resolver-count")
	engine := New([]config.ResolverConfig{
		{
			Name:    "cluster",
			Field:   "label:cluster",
			Command: "sh",
			Args: []string{
				"-c",
				`count=$(cat "$1" 2>/dev/null || echo 0); count=$((count+1)); printf '%s' "$count" > "$1"; exit 1`,
				"--",
				counterFile,
			},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	first := engine.ResolveAlert(alert)
	second := engine.ResolveAlert(alert)
	current = current.Add(defaultFailureCacheTTL + time.Second)
	third := engine.ResolveAlert(alert)

	if len(first.ResolvedLabels) != 0 {
		t.Fatalf("expected no resolved labels on first failure, got %#v", first.ResolvedLabels)
	}
	if len(second.ResolvedLabels) != 0 {
		t.Fatalf("expected no resolved labels on cached failure, got %#v", second.ResolvedLabels)
	}
	if len(third.ResolvedLabels) != 0 {
		t.Fatalf("expected no resolved labels after cache expiry, got %#v", third.ResolvedLabels)
	}

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("reading counter file: %v", err)
	}
	if string(data) != "2" {
		t.Fatalf("expected resolver command to run twice across failure cache expiry, got %q", string(data))
	}
}

func TestResolveAlertUsesCacheWithEnv(t *testing.T) {
	counterDir := t.TempDir()
	counterFile := filepath.Join(counterDir, "resolver-count")

	engine := New([]config.ResolverConfig{
		{
			Name:    "cluster",
			Field:   "label:cluster",
			Command: "sh",
			Args: []string{
				"-c",
				`count=$(cat "$1" 2>/dev/null || echo 0); count=$((count+1)); printf '%s' "$count" > "$1"; printf '%s' "$CACHE_A/$CACHE_B/$VALUE"`,
				"--",
				counterFile,
			},
			Env: map[string]string{
				"CACHE_A": "left",
				"CACHE_B": "right",
				"VALUE":   "{{.Value}}",
			},
			Timeout: time.Second,
		},
	})

	alert := model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	for i := 0; i < 25; i++ {
		resolved := engine.ResolveAlert(alert)
		if got := resolved.ResolvedLabels["cluster"]; got != "left/right/saas-cs-0b" {
			t.Fatalf("expected cached env-backed value, got %q", got)
		}
	}

	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("reading counter file: %v", err)
	}
	count, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parsing counter file %q: %v", string(data), err)
	}
	if count != 1 {
		t.Fatalf("expected resolver command to run once with stable env cache key, got %d", count)
	}
}
