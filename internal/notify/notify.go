package notify

import (
	"fmt"
	"log"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

// send is the notification dispatcher; overridable in tests.
var send = defaultSend

// Engine sends OS notifications for alert changes.
type Engine struct {
	cfg config.NotificationsConfig

	mu          sync.Mutex
	pending     []model.Alert // batch accumulation
	batchTimer  *time.Timer
	batchWindow time.Duration
}

const defaultBatchWindow = 3 * time.Second

func SendNewAlertNotification(alert model.Alert) error {
	title, body := newAlertNotificationContent(alert)
	return send(title, body)
}

func New(cfg config.NotificationsConfig) *Engine {
	return &Engine{
		cfg:         cfg,
		batchWindow: defaultBatchWindow,
	}
}

// OnDiff processes a diff from the poll engine and sends notifications.
func (e *Engine) OnDiff(diff model.Diff) {
	if !e.cfg.Enabled {
		return
	}

	if e.cfg.OnNew && len(diff.New) > 0 {
		e.enqueueBatch(diff.New)
	}

	if e.cfg.OnResolved && len(diff.Resolved) > 0 {
		for _, alert := range diff.Resolved {
			e.sendResolved(alert)
		}
	}
}

func (e *Engine) enqueueBatch(alerts []model.Alert) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.pending = append(e.pending, alerts...)

	if e.batchTimer == nil {
		e.batchTimer = time.AfterFunc(e.batchWindow, e.flushBatch)
	}
}

func (e *Engine) flushBatch() {
	e.mu.Lock()
	pending := e.pending
	e.pending = nil
	e.batchTimer = nil
	e.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	threshold := e.cfg.BatchThreshold
	if threshold <= 0 {
		threshold = 5
	}

	if len(pending) >= threshold {
		// Batch notification
		critCount := 0
		for _, a := range pending {
			if a.Severity == "critical" {
				critCount++
			}
		}
		title := fmt.Sprintf("Foghorn: %d new alerts", len(pending))
		body := fmt.Sprintf("%d critical, %d other", critCount, len(pending)-critCount)
		e.send(title, body)
		return
	}

	// Individual notifications
	for _, alert := range pending {
		e.sendNew(alert)
	}
}

func (e *Engine) sendNew(alert model.Alert) {
	title, body := newAlertNotificationContent(alert)
	e.send(title, body)
}

func (e *Engine) sendResolved(alert model.Alert) {
	title := fmt.Sprintf("[RESOLVED] %s", alert.Name)
	body := fmt.Sprintf("Source: %s", alert.Source)
	e.send(title, body)
}

func (e *Engine) send(title, body string) {
	if err := send(title, body); err != nil {
		log.Printf("notify: failed to send notification: %v", err)
	}
}

func severityLabel(s string) string {
	switch s {
	case "critical":
		return "CRITICAL"
	case "warning":
		return "WARNING"
	case "info":
		return "INFO"
	default:
		return s
	}
}

func newAlertNotificationContent(alert model.Alert) (string, string) {
	title := fmt.Sprintf("[%s] %s", severityLabel(alert.Severity), alert.Name)
	body := ""
	if s := alert.Annotations["summary"]; s != "" {
		body = s
	} else if d := alert.Annotations["description"]; d != "" {
		body = d
	}
	if body == "" {
		body = fmt.Sprintf("Source: %s", alert.Source)
	}
	return title, body
}
