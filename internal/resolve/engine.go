package resolve

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"text/template"
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
	timeout  time.Duration
	cacheTTL time.Duration
}

type cacheEntry struct {
	value     string
	err       error
	expiresAt time.Time
}

type templateData struct {
	Ref         string
	Kind        string
	Name        string
	Value       string
	Alert       model.Alert
	Labels      map[string]string
	Annotations map[string]string
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
			timeout:  timeout,
			cacheTTL: cfg.CacheTTL,
		})
	}
	return engine
}

func (e *Engine) ResolveAlerts(ctx context.Context, alerts []model.Alert) []model.Alert {
	if e == nil || len(e.resolvers) == 0 || len(alerts) == 0 {
		return alerts
	}

	resolved := make([]model.Alert, len(alerts))
	for i, alert := range alerts {
		resolved[i] = e.ResolveAlert(ctx, alert)
	}
	return resolved
}

func (e *Engine) ResolveAlert(ctx context.Context, alert model.Alert) model.Alert {
	if e == nil || len(e.resolvers) == 0 {
		return alert
	}

	out := alert
	for _, item := range e.resolvers {
		raw := resolveRawField(alert, item.field)
		if raw == "" {
			continue
		}

		value, err := e.resolveValue(ctx, item, alert, raw)
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

func (e *Engine) resolveValue(ctx context.Context, item resolver, alert model.Alert, raw string) (string, error) {
	kind, name := config.ResolveFieldRef(item.field)
	data := templateData{
		Ref:         item.field,
		Kind:        kind,
		Name:        name,
		Value:       raw,
		Alert:       alert,
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
	}

	command, err := render(item.command, data)
	if err != nil {
		return "", err
	}

	args := make([]string, 0, len(item.args))
	for _, arg := range item.args {
		rendered, err := render(arg, data)
		if err != nil {
			return "", err
		}
		args = append(args, rendered)
	}

	envKeys := make([]string, 0, len(item.env))
	for key := range item.env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)

	env := make([]string, 0, len(item.env))
	for _, key := range envKeys {
		value := item.env[key]
		rendered, err := render(value, data)
		if err != nil {
			return "", err
		}
		env = append(env, key+"="+rendered)
	}

	cacheKey := item.name + "\x00" + command + "\x00" + strings.Join(args, "\x00") + "\x00" + strings.Join(env, "\x00")
	if cached, ok := e.cache.Load(cacheKey); ok {
		entry := cached.(cacheEntry)
		if entry.expiresAt.IsZero() || timeNow().Before(entry.expiresAt) {
			return entry.value, entry.err
		}
		e.cache.Delete(cacheKey)
	}

	tctx, cancel := context.WithTimeout(ctx, item.timeout)
	defer cancel()

	cmd := exec.CommandContext(tctx, command, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	log.Printf("resolver: executing name=%q field=%q value=%q command=%q args=%q", item.name, item.field, raw, command, args)

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

func render(tmpl string, data templateData) (string, error) {
	t, err := template.New("resolver").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
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
