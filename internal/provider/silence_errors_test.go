package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

func testSilenceRequest() model.SilenceRequest {
	return model.SilenceRequest{
		Matchers: []model.Matcher{{Name: "alertname", Value: "X", IsEqual: true}},
		StartsAt: time.Now(),
		EndsAt:   time.Now().Add(time.Hour),
	}
}

func TestSilenceDeniedReturnsTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	_, err := NewGrafana(grafanaTestSource(server.URL)).Silence(context.Background(), testSilenceRequest())
	var denied *SilenceDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected SilenceDeniedError, got %v", err)
	}
	if denied.Status != 403 || !strings.Contains(denied.Body, "forbidden") || denied.Source != "grafana-main" {
		t.Fatalf("unexpected error %+v", denied)
	}
	if !strings.Contains(err.Error(), "refused: HTTP 403") {
		t.Fatalf("unexpected message %q", err.Error())
	}
}

func TestUnsilenceDeniedReturnsTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	err := NewAlertmanager(config.SourceConfig{Name: "am", Type: "alertmanager", URL: server.URL}).Unsilence(context.Background(), "s1")
	var denied *SilenceDeniedError
	if !errors.As(err, &denied) || denied.Status != 401 {
		t.Fatalf("expected 401 SilenceDeniedError, got %v", err)
	}
}

func TestSilenceServerErrorIsNotDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := NewGrafana(grafanaTestSource(server.URL)).Silence(context.Background(), testSilenceRequest())
	var denied *SilenceDeniedError
	if err == nil || errors.As(err, &denied) {
		t.Fatalf("expected plain error, got %v", err)
	}
}

func TestSilenceDeniedAfterOIDCRetryOnly(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	for _, tc := range []struct {
		name        string
		retryStatus int
		wantDenied  bool
	}{
		{"retry succeeds", http.StatusAccepted, false},
		{"retry refused", http.StatusUnauthorized, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					writeJSON(t, w, map[string]string{
						"device_authorization_endpoint": "http://" + r.Host + "/oauth/device/code",
						"token_endpoint":                "http://" + r.Host + "/oauth/token",
					})
				case "/oauth/device/code":
					writeJSON(t, w, map[string]interface{}{
						"device_code": "d", "user_code": "U", "verification_uri": "https://login.example.test/device",
						"expires_in": 600, "interval": 0,
					})
				case "/oauth/token":
					writeJSON(t, w, map[string]interface{}{
						"access_token": "a", "refresh_token": "r", "token_type": "Bearer", "expires_in": 3600,
					})
				case "/api/v2/alerts":
					writeJSON(t, w, []interface{}{})
				case "/api/v2/silences":
					if posts.Add(1) == 1 {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					w.WriteHeader(tc.retryStatus)
					if tc.retryStatus == http.StatusAccepted {
						w.Write([]byte(`{"silenceID":"s1"}`))
					}
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			am := NewAlertmanager(config.SourceConfig{
				Name: "oidc-am", Type: "alertmanager", URL: server.URL,
				Auth: config.AuthConfig{
					Type: "oidc", Flow: "device", IssuerURL: server.URL, ClientID: "c",
					PersistTokens: testBoolPointer(false),
				},
			})
			defer am.Close()
			if _, err := fetchAfterLogin(t, am); err != nil {
				t.Fatalf("login: %v", err)
			}
			_, err := am.Silence(context.Background(), testSilenceRequest())
			var denied *SilenceDeniedError
			if got := errors.As(err, &denied); got != tc.wantDenied {
				t.Fatalf("denied=%v, want %v (err %v, posts %d)", got, tc.wantDenied, err, posts.Load())
			}
			if tc.wantDenied && denied.Status != 401 {
				t.Fatalf("status %d", denied.Status)
			}
			if !tc.wantDenied && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if posts.Load() != 2 {
				t.Fatalf("expected 2 POSTs, got %d", posts.Load())
			}
		})
	}
}
