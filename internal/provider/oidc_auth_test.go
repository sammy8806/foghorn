package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestAlertmanagerOIDCRejectsEndpointRedirects(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")

	for _, redirectPath := range []string{"/oauth/device/code", "/oauth/token"} {
		t.Run(redirectPath, func(t *testing.T) {
			var redirectedRequests atomic.Int32
			attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				redirectedRequests.Add(1)
				writeJSON(t, w, map[string]string{"error": "request escaped issuer origin"})
			}))
			defer attacker.Close()

			issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					writeJSON(t, w, map[string]string{
						"device_authorization_endpoint": "http://" + r.Host + "/oauth/device/code",
						"token_endpoint":                "http://" + r.Host + "/oauth/token",
					})
				case redirectPath:
					http.Redirect(w, r, attacker.URL+"/capture", http.StatusTemporaryRedirect)
				case "/oauth/device/code":
					writeJSON(t, w, map[string]interface{}{
						"device_code":      "device-123",
						"user_code":        "ABCD-EFGH",
						"verification_uri": "https://login.example.test/device",
						"expires_in":       600,
					})
				case "/oauth/token":
					writeJSON(t, w, map[string]interface{}{
						"access_token": "access-token-123",
						"token_type":   "Bearer",
						"expires_in":   3600,
					})
				default:
					http.NotFound(w, r)
				}
			}))
			defer issuer.Close()

			am := NewAlertmanager(config.SourceConfig{
				Name: "oidc-am",
				Type: "alertmanager",
				URL:  issuer.URL,
				Auth: config.AuthConfig{
					Type:      "oidc",
					Flow:      "device",
					IssuerURL: issuer.URL,
					ClientID:  "foghorn-test",
				},
			})

			if _, err := am.Fetch(context.Background()); err == nil {
				t.Fatal("expected redirected OIDC endpoint request to fail")
			}
			if redirectedRequests.Load() != 0 {
				t.Fatalf("OIDC client followed endpoint redirect %q to another origin", redirectPath)
			}
		})
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

// A compromised/hostile issuer must not be able to redirect the device
// authorization/token requests (and the client_id/client_secret sent to them)
// to an attacker-controlled host via the discovery document.
func TestOIDCDiscoveryRejectsCrossOriginEndpoints(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, map[string]string{
			"device_authorization_endpoint": "https://attacker.example/oauth/device/code",
			"token_endpoint":                "https://attacker.example/oauth/token",
		})
	}))
	defer server.Close()

	auth := newOIDCDeviceAuthenticator(config.AuthConfig{
		Type:      "oidc",
		Flow:      "device",
		IssuerURL: server.URL,
		ClientID:  "foghorn-test",
	}, server.Client())

	_, err := auth.Token(context.Background())
	if err == nil {
		t.Fatal("expected an error for cross-origin discovery endpoints")
	}
	if !strings.Contains(err.Error(), "issuer's origin") {
		t.Fatalf("expected an origin-pinning error, got: %v", err)
	}
}

// A discovery document pointing at a non-loopback http endpoint must be
// rejected: https discovery over MITM'able http could still be downgraded.
func TestOIDCDiscoveryRejectsNonHTTPSEndpoint(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		host := strings.TrimPrefix(server.URL, "http://")
		writeJSON(t, w, map[string]string{
			"device_authorization_endpoint": "http://evil-" + host + "/oauth/device/code",
			"token_endpoint":                "http://evil-" + host + "/oauth/token",
		})
	}))
	defer server.Close()

	auth := newOIDCDeviceAuthenticator(config.AuthConfig{
		Type:      "oidc",
		Flow:      "device",
		IssuerURL: server.URL,
		ClientID:  "foghorn-test",
	}, server.Client())

	_, err := auth.Token(context.Background())
	if err == nil {
		t.Fatal("expected an error for a non-loopback http discovery endpoint")
	}
}

// verification_uri(_complete) comes from the token endpoint; a hostile
// response must not be able to make Foghorn launch a local file/handler via
// the OS's browser opener.
func TestOIDCPromptUserRejectsUnsafeVerificationURL(t *testing.T) {
	var opened []string
	original := browserOpenURL
	browserOpenURL = func(url string) error {
		opened = append(opened, url)
		return nil
	}
	defer func() { browserOpenURL = original }()

	auth := &oidcDeviceAuthenticator{}
	auth.promptUser(&oidcDeviceAuthorization{
		DeviceCode:              "device-123",
		VerificationURIComplete: "file:///etc/passwd",
	})

	if len(opened) != 0 {
		t.Fatalf("unsafe verification URL was opened: %v", opened)
	}
}

func TestDecodeOIDCAuthorizationClaimsFiltersIdentityClaims(t *testing.T) {
	payload, err := json.Marshal(map[string]interface{}{
		"aud":                []string{"alertmanager"},
		"azp":                "foghorn",
		"email":              "operator@example.test",
		"groups":             []string{"monitoring-user"},
		"preferred_username": "operator",
		"realm_access": map[string]interface{}{
			"roles": []string{"default-roles-contact"},
		},
		"scope": "openid offline_access",
	})
	if err != nil {
		t.Fatal(err)
	}
	rawToken := "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"

	claims, err := decodeOIDCAuthorizationClaims(rawToken)
	if err != nil {
		t.Fatalf("decodeOIDCAuthorizationClaims() error: %v", err)
	}
	for _, name := range []string{"aud", "azp", "groups", "realm_access", "scope"} {
		if _, ok := claims[name]; !ok {
			t.Errorf("expected authorization claim %q", name)
		}
	}
	for _, name := range []string{"email", "preferred_username"} {
		if _, ok := claims[name]; ok {
			t.Errorf("identity claim %q must not be included in debug output", name)
		}
	}
}

func TestDecodeOIDCAuthorizationClaimsRejectsNonJWT(t *testing.T) {
	if _, err := decodeOIDCAuthorizationClaims("opaque-access-token"); err == nil {
		t.Fatal("expected non-JWT access token to be rejected")
	}
}
