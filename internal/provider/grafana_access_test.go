package provider

import (
	"context"
	"log"
	"os"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
	"foghorn/internal/silenceaccess"
)

type grafanaFake struct {
	mu       sync.Mutex
	calls    map[string]int
	auth     map[string]string
	status   map[string]int // path -> forced status
	silences []map[string]interface{}
	server   *httptest.Server
}

const (
	permPath   = "/api/access-control/user/permissions"
	rulesPath  = "/api/prometheus/grafana/api/v1/rules"
	folderPath = "/api/search"
)

func newGrafanaFake(t *testing.T) *grafanaFake {
	f := &grafanaFake{calls: map[string]int{}, auth: map[string]string{}, status: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.calls[r.URL.Path]++
		f.auth[r.URL.Path] = r.Header.Get("Authorization")
		st := f.status[r.URL.Path]
		f.mu.Unlock()
		if st != 0 {
			w.WriteHeader(st)
			return
		}
		switch r.URL.Path {
		case permPath:
			json.NewEncoder(w).Encode(map[string][]string{
				"alert.silences:create": {"folders:uid:f1"},
				"alert.silences:write":  {"folders:uid:f1"},
				"alert.silences:read":   {"folders:*"},
			})
		case rulesPath:
			w.Write([]byte(`{"status":"success","data":{"groups":[{"folderUid":"f1","rules":[{"uid":"r1","folderUid":"f1"}]}]}}`))
		case folderPath:
			if r.URL.Query().Get("type") != "dash-folder" {
				t.Errorf("folder search query %q", r.URL.RawQuery)
			}
			w.Write([]byte(`[{"uid":"f0","title":"Top"},{"uid":"f1","folderUid":"f0","title":"Infra"}]`))
		case "/api/alertmanager/grafana/api/v2/silences":
			if r.Method == http.MethodGet {
				json.NewEncoder(w).Encode(f.silences)
				return
			}
			w.WriteHeader(http.StatusForbidden)
		default:
			if strings.HasPrefix(r.URL.Path, "/api/alertmanager/grafana/api/v2/silence/") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *grafanaFake) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[path]
}

func (f *grafanaFake) totalLookupCalls() int {
	return f.count(permPath) + f.count(rulesPath) + f.count(folderPath)
}

func checkedGrafana(f *grafanaFake, sc config.SilenceCheckConfig) *alertmanagerAPI {
	if sc.Granularity == "" {
		on, iv := true, 15*time.Minute
		sc = config.SilenceCheckConfig{Granularity: "folder", OnStart: &on, Interval: &iv, OnDenied: &on}
	}
	return NewGrafana(config.SourceConfig{
		Name: "g", Type: "grafana", URL: f.server.URL,
		Auth:         config.AuthConfig{Type: "bearer", Token: "tok"},
		SilenceCheck: sc,
	})
}

func TestGrafanaRefreshSilenceAccess(t *testing.T) {
	f := newGrafanaFake(t)
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	s := g.SilenceAccess()
	if s.Check != "ok" || s.RuleFolders["r1"] != "f1" || s.FolderParents["f1"] != "f0" || s.FolderTitles["f1"] != "Infra" || !slices.Equal(s.CreateFolders, []string{"f1"}) {
		t.Fatalf("snapshot %+v", s)
	}
	for _, p := range []string{permPath, rulesPath, folderPath} {
		if f.count(p) != 1 || f.auth[p] != "Bearer tok" {
			t.Fatalf("%s: calls %d auth %q", p, f.count(p), f.auth[p])
		}
	}
}

func TestGrafanaRefreshSourceGranularitySkipsRules(t *testing.T) {
	f := newGrafanaFake(t)
	on, iv := true, time.Minute
	g := checkedGrafana(f, config.SilenceCheckConfig{Granularity: "source", OnStart: &on, Interval: &iv, OnDenied: &on})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	if g.SilenceAccess().Check != "ok" || f.count(rulesPath) != 0 || f.count(folderPath) != 0 {
		t.Fatalf("source granularity must only read permissions")
	}
}

func TestGrafanaRefreshRulesForbidden(t *testing.T) {
	f := newGrafanaFake(t)
	f.status[rulesPath] = http.StatusForbidden
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	if s := g.SilenceAccess(); s.Check != "ok" || len(s.RuleFolders) != 0 {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestGrafanaRefreshPermissions404(t *testing.T) {
	f := newGrafanaFake(t)
	f.status[permPath] = http.StatusNotFound
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	if s := g.SilenceAccess(); s.Check != "unsupported" {
		t.Fatalf("check %q", s.Check)
	}
}

func TestGrafanaRefreshPermissions500(t *testing.T) {
	f := newGrafanaFake(t)
	f.status[permPath] = http.StatusInternalServerError
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	if s := g.SilenceAccess(); s.Check != "error" || !strings.Contains(s.CheckError, "500") {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestGrafanaRefreshNotDue(t *testing.T) {
	f := newGrafanaFake(t)
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	now := time.Now()
	g.RefreshSilenceAccess(context.Background(), now)
	before := f.totalLookupCalls()
	g.RefreshSilenceAccess(context.Background(), now.Add(time.Minute))
	if f.totalLookupCalls() != before {
		t.Fatal("lookup ran before the interval")
	}
}

func TestAlertmanagerRefreshIsNoop(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	am := NewAlertmanager(config.SourceConfig{Name: "am", Type: "alertmanager", URL: server.URL})
	am.RefreshSilenceAccess(context.Background(), time.Now())
	if calls != 0 || am.SilenceAccess().Check != "unchecked" {
		t.Fatalf("calls %d snapshot %+v", calls, am.SilenceAccess())
	}
}

func TestSilenceDeniedRecordsDenial(t *testing.T) {
	f := newGrafanaFake(t)
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	_, err := g.Silence(context.Background(), model.SilenceRequest{
		Matchers: []model.Matcher{{Name: silenceaccess.RuleUIDLabel, Value: "r1", IsEqual: true}},
		StartsAt: time.Now(), EndsAt: time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected refusal")
	}
	if d := g.SilenceAccess().Denied; !slices.Equal(d.CreateFolders, []string{"f1"}) || d.Status != 403 {
		t.Fatalf("denied %+v", d)
	}
}

func TestUnsilenceDeniedUsesCachedMatchers(t *testing.T) {
	f := newGrafanaFake(t)
	f.silences = []map[string]interface{}{{
		"id": "s1", "status": map[string]string{"state": "active"},
		"startsAt": "2026-03-31T10:00:00Z", "endsAt": "2026-03-31T14:00:00Z",
		"matchers": []map[string]interface{}{{"name": silenceaccess.RuleUIDLabel, "value": "r1", "isEqual": true}},
	}}
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	if _, err := g.FetchSilences(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := g.Unsilence(context.Background(), "s1"); err == nil {
		t.Fatal("expected refusal")
	}
	if d := g.SilenceAccess().Denied; !slices.Equal(d.WriteFolders, []string{"f1"}) || d.WriteSource {
		t.Fatalf("denied %+v", d)
	}
	g.Unsilence(context.Background(), "unknown")
	if d := g.SilenceAccess().Denied; !d.WriteSource {
		t.Fatalf("unknown id must deny the source: %+v", d)
	}
}

func TestClosedProviderSkipsLookup(t *testing.T) {
	f := newGrafanaFake(t)
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.Close()
	g.RefreshSilenceAccess(context.Background(), time.Now())
	if f.totalLookupCalls() != 0 {
		t.Fatal("closed provider ran a lookup")
	}
}

func captureLog(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	var mu sync.Mutex
	log.SetOutput(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return buf.Write(p) }))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestSilenceAccessIsLogged(t *testing.T) {
	buf := captureLog(t)
	f := newGrafanaFake(t)
	g := checkedGrafana(f, config.SilenceCheckConfig{})
	g.RefreshSilenceAccess(context.Background(), time.Now())
	g.Silence(context.Background(), model.SilenceRequest{
		Matchers: []model.Matcher{{Name: silenceaccess.RuleUIDLabel, Value: "r1", IsEqual: true}},
		StartsAt: time.Now(), EndsAt: time.Now().Add(time.Hour),
	})
	out := buf.String()
	for _, want := range []string{
		"silence check: grafana g: granularity=folder check=ok create=folders[f1] write=folders[f1] read=yes rules=1 folders=2",
		`silence write refused: grafana g HTTP 403; marked create in folder "f1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q\n%s", want, out)
		}
	}
}
