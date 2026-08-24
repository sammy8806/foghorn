package resolve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

const defaultTimeout = 2 * time.Second
const defaultFailureCacheTTL = 5 * time.Second

var timeNow = time.Now

type Engine struct {
	resolvers []resolver
	cache     sync.Map
}

type resolver struct {
	name     string
	field    string
	command  string
	args     []string
	env      map[string]string
	stdin    string
	timeout  time.Duration
	cacheTTL time.Duration
}

type cacheEntry struct {
	value     string
	err       error
	expiresAt time.Time
}

type resolverJSONInput struct {
	Version     int               `json:"version"`
	Ref         string            `json:"ref"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Value       string            `json:"value"`
	Alert       model.Alert       `json:"alert"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

func New(cfgs []config.ResolverConfig) *Engine {
	engine := &Engine{
		resolvers: make([]resolver, 0, len(cfgs)),
	}
	for i, cfg := range cfgs {
		field := strings.TrimSpace(cfg.Field)
		command := strings.TrimSpace(cfg.Command)
		if field == "" || command == "" {
			continue
		}

		name := strings.TrimSpace(cfg.Name)
		if name == "" {
			name = fmt.Sprintf("%s#%d", field, i)
		}

		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}

		engine.resolvers = append(engine.resolvers, resolver{
			name:     name,
			field:    field,
			command:  command,
			args:     append([]string(nil), cfg.Args...),
			env:      cloneStringMap(cfg.Env),
			stdin:    normalizeStdinMode(cfg.Stdin),
			timeout:  timeout,
			cacheTTL: cfg.CacheTTL,
		})
	}
	return engine
}

func (e *Engine) ResolveAlerts(alerts []model.Alert) []model.Alert {
	if e == nil || len(e.resolvers) == 0 || len(alerts) == 0 {
		return alerts
	}

	resolved := make([]model.Alert, len(alerts))
	for i, alert := range alerts {
		resolved[i] = e.ResolveAlert(alert)
	}
	return resolved
}

func (e *Engine) ResolveAlert(alert model.Alert) model.Alert {
	if e == nil || len(e.resolvers) == 0 {
		return alert
	}

	out := alert
	for _, item := range e.resolvers {
		raw := resolveRawField(alert, item.field)
		if raw == "" {
			continue
		}

		value, err := e.resolveValue(item, alert, raw)
		if err != nil || value == "" || value == raw {
			continue
		}

		kind, name := config.ResolveFieldRef(item.field)
		switch kind {
		case "label":
			if out.ResolvedLabels == nil {
				out.ResolvedLabels = make(map[string]string)
			}
			out.ResolvedLabels[name] = value
		case "annotation":
			if out.ResolvedAnnotations == nil {
				out.ResolvedAnnotations = make(map[string]string)
			}
			out.ResolvedAnnotations[name] = value
		case "field":
			if out.ResolvedFields == nil {
				out.ResolvedFields = make(map[string]string)
			}
			out.ResolvedFields[name] = value
		}
	}
	return out
}

func (e *Engine) resolveValue(item resolver, alert model.Alert, raw string) (string, error) {
	kind, name := config.ResolveFieldRef(item.field)
	input, err := resolverInput(item.stdin, resolverJSONInput{
		Version:     1,
		Ref:         item.field,
		Kind:        kind,
		Name:        name,
		Value:       raw,
		Alert:       alert,
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
	})
	if err != nil {
		return "", err
	}

	envKeys := make([]string, 0, len(item.env))
	for key := range item.env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)

	env := make([]string, 0, len(item.env))
	for _, key := range envKeys {
		env = append(env, key+"="+item.env[key])
	}

	cacheKey := resolverCacheKey(item, env, input)
	if cached, ok := e.cache.Load(cacheKey); ok {
		entry := cached.(cacheEntry)
		if entry.expiresAt.IsZero() || timeNow().Before(entry.expiresAt) {
			return entry.value, entry.err
		}
		e.cache.Delete(cacheKey)
	}

	ctx, cancel := context.WithTimeout(context.Background(), item.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, item.command, item.args...)
	cmd.Stdin = bytes.NewReader(input)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	log.Printf("resolver: executing name=%q field=%q value=%q command=%q args=%q", item.name, item.field, raw, item.command, item.args)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("resolver: execution failed name=%q field=%q value=%q err=%v", item.name, item.field, raw, err)
		e.cache.Store(cacheKey, cacheEntry{
			err:       err,
			expiresAt: timeNow().Add(defaultFailureCacheTTL),
		})
		return "", err
	}

	value := strings.TrimSpace(string(output))
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	log.Printf("resolver: execution succeeded name=%q field=%q value=%q resolved=%q", item.name, item.field, raw, value)
	entry := cacheEntry{value: value}
	if item.cacheTTL > 0 {
		entry.expiresAt = timeNow().Add(item.cacheTTL)
	}
	e.cache.Store(cacheKey, entry)
	return value, nil
}

func normalizeStdinMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "value"
	}
	return mode
}

func resolverInput(mode string, data resolverJSONInput) ([]byte, error) {
	switch mode {
	case "value":
		return []byte(data.Value), nil
	case "json":
		return json.Marshal(data)
	default:
		return nil, fmt.Errorf("resolver %q: unsupported stdin mode %q", data.Ref, mode)
	}
}

func resolverCacheKey(item resolver, env []string, input []byte) string {
	var key strings.Builder
	appendCacheKeyPart(&key, item.name)
	appendCacheKeyPart(&key, item.command)
	appendCacheKeyPart(&key, strconv.Itoa(len(item.args)))
	for _, arg := range item.args {
		appendCacheKeyPart(&key, arg)
	}
	appendCacheKeyPart(&key, strconv.Itoa(len(env)))
	for _, variable := range env {
		appendCacheKeyPart(&key, variable)
	}
	appendCacheKeyPart(&key, item.stdin)
	inputDigest := sha256.Sum256(input)
	appendCacheKeyPart(&key, string(inputDigest[:]))
	return key.String()
}

func appendCacheKeyPart(key *strings.Builder, part string) {
	key.WriteString(strconv.Itoa(len(part)))
	key.WriteByte(':')
	key.WriteString(part)
}

func resolveRawField(alert model.Alert, ref string) string {
	kind, name := config.ResolveFieldRef(ref)
	switch kind {
	case "field":
		switch name {
		case "severity":
			return alert.Severity
		case "startsAt":
			return alert.StartsAt.Format(time.RFC3339)
		case "updatedAt":
			return alert.UpdatedAt.Format(time.RFC3339)
		case "source":
			return alert.Source
		case "name":
			return alert.Name
		case "state":
			return alert.State
		default:
			return ""
		}
	case "annotation":
		return alert.Annotations[name]
	default:
		return alert.Labels[name]
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
