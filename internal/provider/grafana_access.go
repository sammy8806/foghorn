package provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"foghorn/internal/model"
	"foghorn/internal/silenceaccess"
)

// SilenceAccess returns the source's silence permission state.
func (a *alertmanagerAPI) SilenceAccess() model.SilenceAccess {
	return a.access.Snapshot()
}

// CanReadSilences is false when a lookup showed the credentials can't read
// silences; the poller then skips FetchSilences.
func (a *alertmanagerAPI) CanReadSilences() bool {
	return a.access.CanReadSilences()
}

// RefreshSilenceAccess runs a Grafana permission lookup when one is due.
// Failures land in the snapshot.
func (a *alertmanagerAPI) RefreshSilenceAccess(ctx context.Context, now time.Time) {
	if a.kind != "grafana" || a.isClosed() || !a.access.CheckDue(now) {
		return
	}
	perms := map[string][]string{}
	if err := a.getGrafanaJSON(ctx, "/api/access-control/user/permissions", &perms); err != nil {
		var se *statusError
		if errors.As(err, &se) && se.status == http.StatusNotFound {
			err = fmt.Errorf("%w: %v", silenceaccess.ErrUnsupported, err)
		}
		if !a.isClosed() {
			a.access.ApplyLookupError(err, now)
		}
		return
	}
	lookup := silenceaccess.Lookup{Permissions: perms}
	if a.access.Granularity() == "folder" {
		if body, err := a.getGrafanaBody(ctx, "/api/prometheus/grafana/api/v1/rules"); err != nil {
			log.Printf("silence check: %s %s rules not readable, folder decisions fall back to source: %v", a.kind, a.cfg.Name, err)
		} else if rules, err := silenceaccess.ParseRules(body); err != nil {
			log.Printf("silence check: %s %s: %v", a.kind, a.cfg.Name, err)
		} else {
			lookup.RuleFolders = rules
		}
		if body, err := a.getGrafanaBody(ctx, "/api/search?type=dash-folder&limit=5000"); err != nil {
			log.Printf("silence check: %s %s folders not readable: %v", a.kind, a.cfg.Name, err)
		} else if parents, titles, err := silenceaccess.ParseFolders(body); err != nil {
			log.Printf("silence check: %s %s: %v", a.kind, a.cfg.Name, err)
		} else {
			lookup.FolderParents, lookup.FolderTitles = parents, titles
		}
	}
	if a.isClosed() {
		return
	}
	a.access.ApplyLookup(lookup, now)
}

type statusError struct {
	status int
	body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.status, e.body)
}

func (a *alertmanagerAPI) getGrafanaBody(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(a.cfg.URL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if err := a.applyAuth(ctx, req); err != nil {
		return nil, err
	}
	resp, err := a.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %w", path, &statusError{status: resp.StatusCode, body: errorBody(resp)})
	}
	return readBodyLimited(resp)
}

func (a *alertmanagerAPI) getGrafanaJSON(ctx context.Context, path string, out any) error {
	body, err := a.getGrafanaBody(ctx, path)
	if err != nil {
		return err
	}
	if err := jsonUnmarshal(body, out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}
	return nil
}

func (a *alertmanagerAPI) isClosed() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.closed
}

// recordSilenceDenied feeds a refused write into the tracker.
func (a *alertmanagerAPI) recordSilenceDenied(op silenceaccess.Op, matchers []model.Matcher, denied *SilenceDeniedError) {
	a.access.RecordDenied(op, matchers, denied.Status, denied.Body, time.Now())
}

// cachedSilenceMatchers returns the matchers of an active silence from the
// last FetchSilences, or nil when unknown.
func (a *alertmanagerAPI) cachedSilenceMatchers(id string) []model.Matcher {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.silenceMatchers[id]
}
