package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

type alertmanagerAPI struct {
	cfg    config.SourceConfig
	client *http.Client
	mu     sync.RWMutex
	health model.ProviderHealth
	apiV2  string
	kind   string
	oidc   *oidcDeviceAuthenticator
	cookie *cookieAuthenticator
}

func NewAlertmanager(cfg config.SourceConfig) *alertmanagerAPI {
	return newAlertmanagerAPI(cfg, "alertmanager", "/api/v2")
}

func NewGrafana(cfg config.SourceConfig) *alertmanagerAPI {
	return newAlertmanagerAPI(cfg, "grafana", "/api/alertmanager/grafana/api/v2")
}

func newAlertmanagerAPI(cfg config.SourceConfig, kind, apiV2 string) *alertmanagerAPI {
	cookieAuth, err := newCookieAuthenticator(cfg)
	if err != nil {
		log.Printf("%s %s cookie auth disabled: %v", kind, cfg.Name, err)
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: withHTTPDebug(nil),
	}
	if cookieAuth != nil {
		// Cookie auth needs to see cross-host redirects itself (to detect and
		// replay a login flow via HandleRedirect below), so it gets a narrower
		// policy than the plain ErrUseLastResponse used otherwise.
		client.Jar = cookieAuth.jar
		client.CheckRedirect = stopCrossDomainCookieRedirect
	} else {
		// Do not follow redirects. Besides preventing POST requests from being
		// rewritten as GETs, this keeps OIDC requests pinned to the endpoints
		// that passed issuer-origin validation.
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &alertmanagerAPI{
		cfg:    cfg,
		client: client,
		apiV2:  apiV2,
		kind:   kind,
		oidc:   newOIDCDeviceAuthenticator(cfg.Auth, client),
		cookie: cookieAuth,
	}
}

func (a *alertmanagerAPI) Name() string          { return a.cfg.Name }
func (a *alertmanagerAPI) Type() string          { return a.kind }
func (a *alertmanagerAPI) SupportsSilence() bool { return true }

func (a *alertmanagerAPI) Health(_ context.Context) model.ProviderHealth {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.health
}

func (a *alertmanagerAPI) Fetch(ctx context.Context) ([]model.Alert, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.endpoint("/alerts"), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("active", "true")
	q.Set("silenced", "true")
	q.Set("inhibited", "true")
	for _, f := range a.cfg.Filters {
		q.Add("filter", f)
	}
	req.URL.RawQuery = q.Encode()

	if err := a.applyAuth(ctx, req); err != nil {
		a.recordError(err)
		return nil, err
	}

	resp, err := a.do(req)
	if err != nil {
		a.recordError(err)
		return nil, fmt.Errorf("fetching alerts from %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.recordError(fmt.Errorf("HTTP %d: %s", resp.StatusCode, errorBody(resp)))
		return nil, fmt.Errorf("%s %s returned HTTP %d", a.kind, a.cfg.Name, resp.StatusCode)
	}

	var raw []amAlert
	if err := decodeJSONResponse(resp, &raw); err != nil {
		return nil, fmt.Errorf("decoding alerts from %s: %w", a.cfg.Name, err)
	}

	alerts := make([]model.Alert, 0, len(raw))
	for _, r := range raw {
		alerts = append(alerts, r.toAlert(a.cfg.Name, a.kind, a.cfg.SeverityLabel))
	}

	a.mu.Lock()
	a.health = model.ProviderHealth{
		Connected:   true,
		LastSuccess: time.Now(),
		AlertCount:  len(alerts),
	}
	a.mu.Unlock()

	return alerts, nil
}

func (a *alertmanagerAPI) Silence(ctx context.Context, req model.SilenceRequest) (string, error) {
	body := amSilenceRequest{
		ID:        req.ID,
		Matchers:  make([]amMatcher, len(req.Matchers)),
		StartsAt:  req.StartsAt.Format(time.RFC3339),
		EndsAt:    req.EndsAt.Format(time.RFC3339),
		CreatedBy: req.CreatedBy,
		Comment:   req.Comment,
	}
	for i, m := range req.Matchers {
		body.Matchers[i] = amMatcher{
			Name:    m.Name,
			Value:   m.Value,
			IsRegex: m.IsRegex,
			IsEqual: m.IsEqual,
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.endpoint("/silences"), bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := a.applyAuth(ctx, httpReq); err != nil {
		return "", err
	}

	if HTTPDebugEnabled() {
		log.Printf("silence: %s %s POST %s body=%s", a.kind, a.cfg.Name, httpReq.URL.Redacted(), string(jsonBody))
	}

	resp, err := a.do(httpReq)
	if err != nil {
		return "", fmt.Errorf("creating silence on %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	respBody, readErr := readBodyLimited(resp)
	if readErr != nil {
		return "", fmt.Errorf("reading silence response from %s: %w", a.cfg.Name, readErr)
	}
	if HTTPDebugEnabled() {
		log.Printf("silence: %s %s response HTTP %d body=%s", a.kind, a.cfg.Name, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("silence on %s %s returned HTTP %d: %s", a.kind, a.cfg.Name, resp.StatusCode, string(respBody))
	}

	var result struct {
		SilenceID string `json:"silenceID"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decoding silence response from %s (HTTP %d, body %q): %w", a.cfg.Name, resp.StatusCode, strings.TrimSpace(string(respBody)), err)
	}
	if strings.TrimSpace(result.SilenceID) == "" {
		// A 2xx with no silenceID means the silence was not created (e.g. the
		// request was rewritten by a proxy). Treat it as a failure rather than
		// reporting a phantom success to the caller.
		return "", fmt.Errorf("silence on %s %s returned HTTP %d but no silence ID; body: %s", a.kind, a.cfg.Name, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return result.SilenceID, nil
}

func (a *alertmanagerAPI) Unsilence(ctx context.Context, silenceID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", a.endpoint("/silence/"+url.PathEscape(silenceID)), nil)
	if err != nil {
		return err
	}
	if err := a.applyAuth(ctx, req); err != nil {
		return err
	}

	resp, err := a.do(req)
	if err != nil {
		return fmt.Errorf("deleting silence on %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete silence on %s %s returned HTTP %d", a.kind, a.cfg.Name, resp.StatusCode)
	}
	return nil
}

func (a *alertmanagerAPI) FetchSilences(ctx context.Context) ([]model.SilenceInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.endpoint("/silences"), nil)
	if err != nil {
		return nil, err
	}
	if err := a.applyAuth(ctx, req); err != nil {
		return nil, err
	}

	resp, err := a.do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching silences from %s: %w", a.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s silences returned HTTP %d: %s", a.kind, a.cfg.Name, resp.StatusCode, errorBody(resp))
	}

	var raw []amSilence
	if err := decodeJSONResponse(resp, &raw); err != nil {
		return nil, fmt.Errorf("decoding silences from %s: %w", a.cfg.Name, err)
	}

	var silences []model.SilenceInfo
	for _, s := range raw {
		if s.Status.State != "active" {
			continue
		}
		startsAt, _ := time.Parse(time.RFC3339, s.StartsAt)
		endsAt, _ := time.Parse(time.RFC3339, s.EndsAt)
		matchers := make([]model.Matcher, 0, len(s.Matchers))
		for _, m := range s.Matchers {
			matchers = append(matchers, model.Matcher{
				Name:    m.Name,
				Value:   m.Value,
				IsRegex: m.IsRegex,
				IsEqual: m.IsEqual,
			})
		}
		silences = append(silences, model.SilenceInfo{
			ID:        s.ID,
			CreatedBy: s.CreatedBy,
			Comment:   s.Comment,
			StartsAt:  startsAt,
			EndsAt:    endsAt,
			Matchers:  matchers,
		})
	}
	return silences, nil
}

func (a *alertmanagerAPI) do(req *http.Request) (*http.Response, error) {
	resp, err := a.client.Do(req)
	if err != nil || resp == nil || a.cookie == nil || !isRedirect(resp.StatusCode) {
		return resp, err
	}
	location := resp.Header.Get("Location")
	if location == "" {
		return resp, err
	}
	loginURL, parseErr := req.URL.Parse(location)
	if parseErr != nil || !isCrossHost(req.URL, loginURL) {
		return resp, err
	}
	resp.Body.Close()
	if err := a.cookie.HandleRedirect(req.Context(), loginURL.String(), req.URL.String()); err != nil {
		return nil, err
	}
	retry := req.Clone(req.Context())
	if req.Body != nil {
		if req.GetBody == nil {
			return nil, fmt.Errorf("%s %s requires cookie login but request body cannot be replayed", a.kind, a.cfg.Name)
		}
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		retry.Body = body
	}
	return a.client.Do(retry)
}

func stopCrossDomainCookieRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if isCrossHost(via[len(via)-1].URL, req.URL) {
		return http.ErrUseLastResponse
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently || status == http.StatusFound || status == http.StatusSeeOther || status == http.StatusTemporaryRedirect || status == http.StatusPermanentRedirect
}

func isCrossHost(from, to *url.URL) bool {
	if from == nil || to == nil {
		return false
	}
	return !strings.EqualFold(from.Host, to.Host)
}

func (a *alertmanagerAPI) endpoint(path string) string {
	return strings.TrimRight(a.cfg.URL, "/") + a.apiV2 + path
}

func (a *alertmanagerAPI) applyAuth(ctx context.Context, req *http.Request) error {
	if a.oidc != nil {
		return a.oidc.Apply(ctx, req)
	}
	applyAuth(req, a.cfg.Auth)
	return nil
}

func (a *alertmanagerAPI) recordError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.health.Connected = false
	a.health.LastError = err.Error()
	a.health.ErrorCount++
}

// --- Alertmanager v2 API response types ---

type amAlert struct {
	Fingerprint  string            `json:"fingerprint"`
	StartsAt     string            `json:"startsAt"`
	UpdatedAt    string            `json:"updatedAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Status       amAlertStatus     `json:"status"`
	Receivers    []amReceiver      `json:"receivers"`
}

type amAlertStatus struct {
	State       string   `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
	MutedBy     []string `json:"mutedBy"`
}

type amReceiver struct {
	Name string `json:"name"`
}

type amSilence struct {
	ID        string          `json:"id"`
	CreatedBy string          `json:"createdBy"`
	Comment   string          `json:"comment"`
	StartsAt  string          `json:"startsAt"`
	EndsAt    string          `json:"endsAt"`
	Matchers  []amMatcher     `json:"matchers"`
	Status    amSilenceStatus `json:"status"`
}

type amSilenceStatus struct {
	State string `json:"state"`
}

type amSilenceRequest struct {
	ID        string      `json:"id,omitempty"`
	Matchers  []amMatcher `json:"matchers"`
	StartsAt  string      `json:"startsAt"`
	EndsAt    string      `json:"endsAt"`
	CreatedBy string      `json:"createdBy"`
	Comment   string      `json:"comment"`
}

type amMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

func (r amAlert) toAlert(source, sourceType, severityLabel string) model.Alert {
	startsAt, _ := time.Parse(time.RFC3339, r.StartsAt)
	updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)

	receivers := make([]string, len(r.Receivers))
	for i, recv := range r.Receivers {
		receivers[i] = recv.Name
	}

	return model.Alert{
		ID:           r.Fingerprint,
		Source:       source,
		SourceType:   sourceType,
		Name:         r.Labels["alertname"],
		Severity:     severityFromLabels(r.Labels, severityLabel),
		State:        r.Status.State,
		Labels:       r.Labels,
		Annotations:  r.Annotations,
		StartsAt:     startsAt,
		UpdatedAt:    updatedAt,
		GeneratorURL: sanitizeRemoteURL(r.GeneratorURL),
		SilencedBy:   r.Status.SilencedBy,
		InhibitedBy:  r.Status.InhibitedBy,
		Receivers:    receivers,
	}
}

func severityFromLabels(labels map[string]string, severityLabel string) string {
	if value := strings.TrimSpace(labels[severityLabel]); value != "" {
		return value
	}
	return strings.TrimSpace(labels["severity"])
}
