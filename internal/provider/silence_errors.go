package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SilenceDeniedError reports a silence write the source refused (401/403
// after any auth retry). Other failures are never wrapped in it.
type SilenceDeniedError struct {
	Source string
	Kind   string
	Status int    // 401 or 403
	Body   string // trimmed response body, max 300 bytes
}

func (e *SilenceDeniedError) Error() string {
	msg := fmt.Sprintf("silence on %s %s refused: HTTP %d", e.Kind, e.Source, e.Status)
	if e.Body != "" {
		msg += ": " + e.Body
	}
	return msg
}

func isDeniedStatus(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}

func (a *alertmanagerAPI) deniedError(status int, body []byte) *SilenceDeniedError {
	trimmed := strings.TrimSpace(string(body))
	// Grafana wraps the reason in JSON; keep only its message.
	var grafana struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &grafana) == nil && strings.TrimSpace(grafana.Message) != "" {
		trimmed = strings.TrimSpace(grafana.Message)
	}
	if len(trimmed) > 300 {
		trimmed = trimmed[:300]
	}
	return &SilenceDeniedError{Source: a.cfg.Name, Kind: a.kind, Status: status, Body: trimmed}
}
