package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"foghorn/internal/config"
)

func TestAlertmanagerOIDCDeviceFlowUsesAccessToken(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	var alertAuth string
	var deviceAuthBody string
	var tokenPollBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/oauth/device/code",
				"token_endpoint":                "http://" + r.Host + "/oauth/token",
			})
		case "/oauth/device/code":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST device auth, got %s", r.Method)
			}
			body := readFormBody(t, r)
			deviceAuthBody = body
			writeJSON(t, w, map[string]interface{}{
				"device_code":               "device-123",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          "https://login.example.test/device",
				"verification_uri_complete": "https://login.example.test/device?user_code=ABCD-EFGH",
				"expires_in":                600,
				"interval":                  0,
			})
		case "/oauth/token":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST token, got %s", r.Method)
			}
			tokenPollBody = readFormBody(t, r)
			writeJSON(t, w, map[string]interface{}{
				"access_token": "access-token-123",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/api/v2/alerts":
			alertAuth = r.Header.Get("Authorization")
			writeJSON(t, w, []map[string]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "oidc-am",
		Type: "alertmanager",
		URL:  server.URL,
		Auth: config.AuthConfig{
			Type:      "oidc",
			Flow:      "device",
			IssuerURL: server.URL,
			ClientID:  "foghorn-test",
			Scopes:    []string{"openid", "profile"},
		},
	}
	am := NewAlertmanager(cfg)

	_, err := am.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if alertAuth != "Bearer access-token-123" {
		t.Fatalf("expected Alertmanager bearer auth, got %q", alertAuth)
	}
	if !strings.Contains(deviceAuthBody, "client_id=foghorn-test") || !strings.Contains(deviceAuthBody, "scope=openid+profile") {
		t.Fatalf("device authorization body did not include client_id/scope: %s", deviceAuthBody)
	}
	if !strings.Contains(tokenPollBody, "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code") || !strings.Contains(tokenPollBody, "device_code=device-123") {
		t.Fatalf("token poll body did not include device grant/device_code: %s", tokenPollBody)
	}
}

func TestAlertmanagerOIDCReusesCachedToken(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	deviceAuthRequests := 0
	tokenRequests := 0
	alertRequests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/oauth/device/code",
				"token_endpoint":                "http://" + r.Host + "/oauth/token",
			})
		case "/oauth/device/code":
			deviceAuthRequests++
			writeJSON(t, w, map[string]interface{}{
				"device_code":      "device-123",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://login.example.test/device",
				"expires_in":       600,
				"interval":         0,
			})
		case "/oauth/token":
			tokenRequests++
			writeJSON(t, w, map[string]interface{}{
				"access_token": "cached-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/api/v2/alerts":
			alertRequests++
			if got := r.Header.Get("Authorization"); got != "Bearer cached-token" {
				t.Fatalf("request %d authorization = %q", alertRequests, got)
			}
			writeJSON(t, w, []map[string]interface{}{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := config.SourceConfig{
		Name: "oidc-am",
		Type: "alertmanager",
		URL:  server.URL,
		Auth: config.AuthConfig{
			Type:      "oidc",
			Flow:      "device",
			IssuerURL: server.URL,
			ClientID:  "foghorn-test",
		},
	}
	am := NewAlertmanager(cfg)

	for i := 0; i < 2; i++ {
		if _, err := am.Fetch(context.Background()); err != nil {
			t.Fatalf("Fetch #%d error: %v", i+1, err)
		}
	}
	if deviceAuthRequests != 1 || tokenRequests != 1 {
		t.Fatalf("expected one OIDC login, got device=%d token=%d", deviceAuthRequests, tokenRequests)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func readFormBody(t *testing.T, r *http.Request) string {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}
	return r.Form.Encode()
}

func TestOIDCDeviceAuthFailsBeforeExpiry(t *testing.T) {
	auth := newOIDCDeviceAuthenticator(config.AuthConfig{
		Type:      "oidc",
		Flow:      "device",
		IssuerURL: "https://login.example.test",
		ClientID:  "foghorn-test",
	}, &http.Client{Timeout: time.Second})
	if auth == nil {
		t.Fatal("expected OIDC authenticator")
	}
}
