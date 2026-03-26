package action

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"text/template"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

type commandSpec struct {
	name string
	args []string
}

// Engine matches alerts against configured actions and executes them.
type Engine struct {
	actions []config.ActionConfig
}

func New(actions []config.ActionConfig) *Engine {
	return &Engine{actions: actions}
}

// ActionsForAlert returns all actions that match the given alert.
func (e *Engine) ActionsForAlert(alert model.Alert) []config.ActionConfig {
	var matched []config.ActionConfig
	for _, a := range e.actions {
		if matchesAlert(a.Match, alert) {
			matched = append(matched, a)
		}
	}
	return matched
}

// Execute runs an action for a given alert.
func (e *Engine) Execute(action config.ActionConfig, alert model.Alert) (string, error) {
	switch action.Action.Type {
	case "url":
		url, err := renderTemplate(action.Action.Template, alert)
		if err != nil {
			return "", fmt.Errorf("rendering URL template: %w", err)
		}
		return url, openURL(url)

	case "shell":
		cmd, err := renderTemplate(action.Action.Command, alert)
		if err != nil {
			return "", fmt.Errorf("rendering command template: %w", err)
		}
		return cmd, runShell(cmd, action.Action.Terminal)

	case "clipboard":
		text, err := renderTemplate(action.Action.Template, alert)
		if err != nil {
			return "", fmt.Errorf("rendering clipboard template: %w", err)
		}
		return text, copyToClipboard(text)

	default:
		return "", fmt.Errorf("unknown action type %q", action.Action.Type)
	}
}

func matchesAlert(matchLabels map[string]string, alert model.Alert) bool {
	for k, v := range matchLabels {
		if alert.Labels[k] != v {
			return false
		}
	}
	return true
}

func renderTemplate(tmpl string, alert model.Alert) (string, error) {
	t, err := template.New("action").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := map[string]interface{}{
		"Alert":        alert,
		"Labels":       alert.Labels,
		"Annotations":  alert.Annotations,
		"Name":         alert.Name,
		"Source":       alert.Source,
		"Severity":     alert.Severity,
		"GeneratorURL": alert.GeneratorURL,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// These are variables so tests can override them without spawning real processes.
var openURL = func(url string) error {
	spec, err := browserOpenCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	return exec.Command(spec.name, spec.args...).Start()
}

var runShell = func(cmd string, _ bool) error {
	return exec.Command("sh", "-c", cmd).Start()
}

var copyToClipboard = func(text string) error {
	spec, err := clipboardCommand(runtime.GOOS, exec.LookPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(spec.name, spec.args...)
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

func browserOpenCommand(goos, url string) (commandSpec, error) {
	switch goos {
	case "darwin":
		return commandSpec{name: "open", args: []string{url}}, nil
	case "linux":
		return commandSpec{name: "xdg-open", args: []string{url}}, nil
	case "windows":
		return commandSpec{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", url}}, nil
	default:
		return commandSpec{}, fmt.Errorf("opening URLs is not supported on %s", goos)
	}
}

func clipboardCommand(goos string, lookPath func(string) (string, error)) (commandSpec, error) {
	candidates := clipboardCommandCandidates(goos)
	for _, candidate := range candidates {
		if _, err := lookPath(candidate.name); err == nil {
			return candidate, nil
		}
	}

	if len(candidates) == 0 {
		return commandSpec{}, fmt.Errorf("clipboard actions are not supported on %s", goos)
	}

	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.name)
	}
	return commandSpec{}, fmt.Errorf("no clipboard command found on %s (tried: %v)", goos, names)
}

func clipboardCommandCandidates(goos string) []commandSpec {
	switch goos {
	case "darwin":
		return []commandSpec{{name: "pbcopy"}}
	case "linux":
		return []commandSpec{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}
	case "windows":
		return []commandSpec{{name: "clip"}}
	default:
		return nil
	}
}
