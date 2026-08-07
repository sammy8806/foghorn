package notify

import (
	"strings"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func testSeverityConfig(t *testing.T) config.NormalizedSeverityConfig {
	t.Helper()
	severities, err := config.NormalizeSeverityConfig(config.DefaultSeverityConfig())
	if err != nil {
		t.Fatalf("NormalizeSeverityConfig() error: %v", err)
	}
	return severities
}

func makeAlert(name, severity string) model.Alert {
	return model.Alert{
		ID:       name,
		Source:   "test",
		Name:     name,
		Severity: severity,
		State:    "active",
		Labels:   map[string]string{"alertname": name, "severity": severity},
		Annotations: map[string]string{
			"summary": "Test alert summary for " + name,
		},
	}
}

func TestBatchingBelowThreshold(t *testing.T) {
	var sent []string
	original := send
	defer func() { send = original }()
	send = func(title, _ string) error {
		sent = append(sent, title)
		return nil
	}

	cfg := config.NotificationsConfig{
		Enabled:        true,
		OnNew:          true,
		BatchThreshold: 5,
	}
	e := New(cfg, testSeverityConfig(t))
	e.batchWindow = 50 * time.Millisecond

	diff := model.Diff{
		New: []model.Alert{
			makeAlert("Alert1", "critical"),
			makeAlert("Alert2", "warning"),
		},
	}
	e.OnDiff(diff)

	time.Sleep(200 * time.Millisecond)

	if len(sent) != 2 {
		t.Errorf("expected 2 individual notifications, got %d", len(sent))
	}
}

func TestBatchingAboveThreshold(t *testing.T) {
	var sent []string
	original := send
	defer func() { send = original }()
	send = func(title, _ string) error {
		sent = append(sent, title)
		return nil
	}

	cfg := config.NotificationsConfig{
		Enabled:        true,
		OnNew:          true,
		BatchThreshold: 3,
	}
	e := New(cfg, testSeverityConfig(t))
	e.batchWindow = 50 * time.Millisecond

	diff := model.Diff{
		New: []model.Alert{
			makeAlert("Alert1", "critical"),
			makeAlert("Alert2", "warning"),
			makeAlert("Alert3", "warning"),
			makeAlert("Alert4", "info"),
		},
	}
	e.OnDiff(diff)

	time.Sleep(200 * time.Millisecond)

	if len(sent) != 1 {
		t.Fatalf("expected 1 batched notification, got %d: %v", len(sent), sent)
	}
	if sent[0] != "Foghorn: 4 new alerts" {
		t.Errorf("unexpected batch title: %q", sent[0])
	}
}

func TestNotificationsDisabled(t *testing.T) {
	var sent []string
	original := send
	defer func() { send = original }()
	send = func(title, _ string) error {
		sent = append(sent, title)
		return nil
	}

	cfg := config.NotificationsConfig{Enabled: false}
	e := New(cfg, testSeverityConfig(t))

	e.OnDiff(model.Diff{
		New: []model.Alert{makeAlert("Alert1", "critical")},
	})

	time.Sleep(100 * time.Millisecond)

	if len(sent) != 0 {
		t.Errorf("expected no notifications when disabled, got %d", len(sent))
	}
}

func TestSendNewAlertNotification(t *testing.T) {
	originalSend := send
	defer func() {
		send = originalSend
	}()

	var gotTitle string
	var gotBody string
	send = func(title, body string) error {
		gotTitle = title
		gotBody = body
		return nil
	}

	if err := SendNewAlertNotification(makeAlert("Alert1", "critical")); err != nil {
		t.Fatalf("SendNewAlertNotification() error: %v", err)
	}

	if gotTitle != "[CRITICAL] Alert1" {
		t.Fatalf("unexpected title: %q", gotTitle)
	}
	if gotBody != "Test alert summary for Alert1" {
		t.Fatalf("unexpected body: %q", gotBody)
	}
}

func TestSanitizeNotificationText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain text is untouched", "Disk usage at 95%", "Disk usage at 95%"},
		{"strips markup", `Click <a href="http://evil.example">here</a>`, "Click here"},
		{"strips bare tags", "<b>urgent</b>", "urgent"},
		{"newlines become spaces", "line one\nline two", "line one line two"},
		{"drops control characters", "bell\x07text", "belltext"},
		{"trims", "  padded  ", "padded"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeNotificationText(tc.in); got != tc.want {
				t.Fatalf("sanitizeNotificationText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeNotificationTextTruncates(t *testing.T) {
	long := strings.Repeat("a", maxNotificationTextLen+50)
	got := sanitizeNotificationText(long)
	if len([]rune(got)) != maxNotificationTextLen+1 {
		t.Fatalf("expected truncation to %d runes plus ellipsis, got %d", maxNotificationTextLen, len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected an ellipsis suffix, got %q", got)
	}
}
