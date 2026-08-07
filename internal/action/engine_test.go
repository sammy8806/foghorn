package action

import (
	"errors"
	"reflect"
	"testing"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func makeAlert(name, severity, cluster string) model.Alert {
	return model.Alert{
		ID:       name,
		Source:   "test",
		Name:     name,
		Severity: severity,
		Labels: map[string]string{
			"alertname": name,
			"severity":  severity,
			"cluster":   cluster,
		},
		Annotations:  map[string]string{"summary": "Test summary"},
		GeneratorURL: "http://prometheus:9090/graph?g0.expr=up",
	}
}

func TestActionsForAlert_Match(t *testing.T) {
	actions := []config.ActionConfig{
		{
			Name:  "Runbook",
			Match: map[string]string{"severity": "critical"},
			Action: config.ActionDef{
				Type:     "url",
				Template: "https://runbooks.example.com/{{.Name}}",
			},
		},
		{
			Name:  "Dashboard",
			Match: map[string]string{"cluster": "prod"},
			Action: config.ActionDef{
				Type:     "url",
				Template: "https://grafana.example.com/{{.Labels.cluster}}",
			},
		},
	}

	e := New(actions)
	alert := makeAlert("HighCPU", "critical", "prod")

	matched := e.ActionsForAlert(alert)
	if len(matched) != 2 {
		t.Errorf("expected 2 matched actions, got %d", len(matched))
	}
}

func TestActionsForAlert_NoMatch(t *testing.T) {
	actions := []config.ActionConfig{
		{
			Name:   "CriticalOnly",
			Match:  map[string]string{"severity": "critical"},
			Action: config.ActionDef{Type: "url", Template: "http://example.com"},
		},
	}

	e := New(actions)
	alert := makeAlert("LowDisk", "warning", "dev")

	matched := e.ActionsForAlert(alert)
	if len(matched) != 0 {
		t.Errorf("expected 0 matched actions, got %d", len(matched))
	}
}

func TestRenderTemplate(t *testing.T) {
	alert := makeAlert("TargetDown", "critical", "saas-cs-0b")
	result, err := renderTemplate("https://runbooks.example.com/{{.Name}}?cluster={{.Labels.cluster}}", alert)
	if err != nil {
		t.Fatalf("renderTemplate() error: %v", err)
	}
	expected := "https://runbooks.example.com/TargetDown?cluster=saas-cs-0b"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRenderTemplate_InvalidTemplate(t *testing.T) {
	alert := makeAlert("Alert", "warning", "prod")
	_, err := renderTemplate("{{.Invalid.Field.That.Does.Not.Exist}}", alert)
	// template execution may succeed with zero value, not error — just verify no panic
	_ = err
}

func TestMatchesAlert_EmptyMatch(t *testing.T) {
	// Empty match should match everything
	alert := makeAlert("Any", "info", "any")
	if !matchesAlert(map[string]string{}, alert) {
		t.Error("empty match should match all alerts")
	}
}

func TestExecute_ClipboardAction(t *testing.T) {
	// Override copyToClipboard to avoid needing pbcopy
	original := copyToClipboard
	defer func() { copyToClipboard = original }()

	var copied string
	copyToClipboard = func(text string) error {
		copied = text
		return nil
	}

	e := New(nil)
	action := config.ActionConfig{
		Action: config.ActionDef{
			Type:     "clipboard",
			Template: "{{.Name}} on {{.Source}}",
		},
	}
	alert := makeAlert("HighMem", "warning", "prod")

	result, err := e.Execute(action, alert)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result != "HighMem on test" {
		t.Errorf("expected 'HighMem on test', got %q", result)
	}
	if copied != "HighMem on test" {
		t.Errorf("expected clipboard 'HighMem on test', got %q", copied)
	}
}

func TestBrowserOpenCommand(t *testing.T) {
	tests := []struct {
		goos string
		want commandSpec
	}{
		{goos: "darwin", want: commandSpec{name: "open", args: []string{"https://example.com"}}},
		{goos: "linux", want: commandSpec{name: "xdg-open", args: []string{"https://example.com"}}},
		{goos: "windows", want: commandSpec{name: "rundll32", args: []string{"url.dll,FileProtocolHandler", "https://example.com"}}},
	}

	for _, tt := range tests {
		got, err := browserOpenCommand(tt.goos, "https://example.com")
		if err != nil {
			t.Fatalf("browserOpenCommand(%q) error: %v", tt.goos, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("browserOpenCommand(%q) = %#v, want %#v", tt.goos, got, tt.want)
		}
	}
}

func TestClipboardCommand_LinuxFallbacks(t *testing.T) {
	lookups := map[string]error{
		"wl-copy": errors.New("missing"),
		"xclip":   nil,
		"xsel":    nil,
	}

	got, err := clipboardCommand("linux", func(name string) (string, error) {
		if err, ok := lookups[name]; ok {
			if err != nil {
				return "", err
			}
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("missing")
	})
	if err != nil {
		t.Fatalf("clipboardCommand(linux) error: %v", err)
	}

	want := commandSpec{name: "xclip", args: []string{"-selection", "clipboard"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("clipboardCommand(linux) = %#v, want %#v", got, want)
	}
}

func TestClipboardCommand_Unsupported(t *testing.T) {
	_, err := clipboardCommand("plan9", func(string) (string, error) {
		return "", errors.New("missing")
	})
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

// URL action templates interpolate alert fields from the remote source, so the
// rendered URL has to be scheme-checked before it reaches the platform opener.
func TestExecute_URLActionRejectsUnsafeSchemes(t *testing.T) {
	original := openURL
	defer func() { openURL = original }()

	var opened []string
	openURL = func(url string) error {
		opened = append(opened, url)
		return nil
	}

	cases := []struct {
		name     string
		template string
		wantOpen bool
	}{
		{"https", "https://runbooks.example.com/{{.Name}}", true},
		{"http", "http://runbooks.example.com/{{.Name}}", true},
		{"mailto", "mailto:oncall@example.com?subject={{.Name}}", true},
		{"file from annotation", "{{.Annotations.evil}}", false},
		{"javascript", "javascript:alert(1)", false},
		{"custom handler", "ms-msdt:/id", false},
		{"no host", "https:///nowhere", false},
		{"empty", "{{.Annotations.missing}}", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opened = nil
			alert := makeAlert("HighMem", "warning", "prod")
			alert.Annotations["evil"] = "file:///etc/passwd"

			action := config.ActionConfig{
				Action: config.ActionDef{Type: "url", Template: tc.template},
			}
			_, err := New(nil).Execute(action, alert)

			if tc.wantOpen {
				if err != nil {
					t.Fatalf("Execute() error: %v", err)
				}
				if len(opened) != 1 {
					t.Fatalf("expected the URL to be opened, got %v", opened)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error for template %q", tc.template)
			}
			if len(opened) != 0 {
				t.Fatalf("unsafe URL was handed to the opener: %v", opened)
			}
		})
	}
}
