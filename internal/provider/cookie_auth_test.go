package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"foghorn/internal/config"
)

func TestAlertmanagerCookieAuthLogsInWhenRedirectedToOtherDomain(t *testing.T) {
	var loginURL string
	alertRequests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "am_session", Value: "session-123", Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/api/v2/alerts":
			alertRequests++
			if r.Header.Get("Cookie") != "am_session=session-123" {
				http.Redirect(w, r, "https://sso.example.test/login?continue=/api/v2/alerts", http.StatusFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "cookie-am",
		Type: "alertmanager",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: "cookie", CookieFile: t.TempDir() + "/cookies.json"},
	}
	am := NewAlertmanager(cfg)
	am.cookie.login = func(ctx context.Context, redirectURL string, returnURL string) error {
		loginURL = redirectURL
		parsed, err := url.Parse(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		am.cookie.jar.SetCookies(parsed, []*http.Cookie{{Name: "am_session", Value: "session-123", Path: "/"}})
		return nil
	}

	alerts, err := am.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts, got %d", len(alerts))
	}
	if loginURL != "https://sso.example.test/login?continue=/api/v2/alerts" {
		t.Fatalf("expected cross-domain redirect login URL, got %q", loginURL)
	}
	if alertRequests != 2 {
		t.Fatalf("expected fetch to retry after cookie login, got %d requests", alertRequests)
	}
}

func TestAlertmanagerCookieAuthUsesStoredCookies(t *testing.T) {
	alertRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts" {
			http.NotFound(w, r)
			return
		}
		alertRequests++
		if got := r.Header.Get("Cookie"); got != "am_session=session-123" {
			t.Fatalf("expected stored cookie, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "cookie-am",
		Type: "alertmanager",
		URL:  server.URL,
		Auth: config.AuthConfig{Type: "cookie", CookieFile: t.TempDir() + "/cookies.json"},
	}
	am := NewAlertmanager(cfg)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	am.cookie.jar.SetCookies(parsed, []*http.Cookie{{Name: "am_session", Value: "session-123", Path: "/"}})

	if _, err := am.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if alertRequests != 1 {
		t.Fatalf("expected one request with stored cookie, got %d", alertRequests)
	}
}
