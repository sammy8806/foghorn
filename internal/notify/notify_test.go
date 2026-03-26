package notify

import (
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

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
	e := New(cfg)
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
	e := New(cfg)
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
	e := New(cfg)

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
