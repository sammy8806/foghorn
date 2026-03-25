package action

import (
	"bytes"
	"fmt"
	"os/exec"
	"text/template"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

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
		"Alert":       alert,
		"Labels":      alert.Labels,
		"Annotations": alert.Annotations,
		"Name":        alert.Name,
		"Source":      alert.Source,
		"Severity":    alert.Severity,
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
	return exec.Command("open", url).Start()
}

var runShell = func(cmd string, _ bool) error {
	return exec.Command("sh", "-c", cmd).Start()
}

var copyToClipboard = func(text string) error {
	pbcopy := exec.Command("pbcopy")
	pbcopy.Stdin = bytes.NewBufferString(text)
	return pbcopy.Run()
}
