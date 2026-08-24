package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
			Type:          "oidc",
			Flow:          "device",
			IssuerURL:     server.URL,
			ClientID:      "foghorn-test",
			Scopes:        []string{"openid", "profile"},
			PersistTokens: testBoolPointer(false),
		},
	}
	am := NewAlertmanager(cfg)

	if _, err := fetchAfterLogin(t, am); err != nil {
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
			Type:          "oidc",
			Flow:          "device",
			IssuerURL:     server.URL,
			ClientID:      "foghorn-test",
			PersistTokens: testBoolPointer(false),
		},
	}
	am := NewAlertmanager(cfg)

	for i := 0; i < 2; i++ {
		if _, err := fetchAfterLogin(t, am); err != nil {
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
					Type:          "oidc",
					Flow:          "device",
					IssuerURL:     issuer.URL,
					ClientID:      "foghorn-test",
					PersistTokens: testBoolPointer(false),
				},
			})

			if _, err := fetchAfterLogin(t, am); err == nil {
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

func testBoolPointer(value bool) *bool { return &value }

// fetchAfterLogin polls a source the way the engine does. The first fetch that
// triggers a device login fails fast with ErrLoginPending while the sign-in runs
// in the background, so a caller retries instead of holding the fetch open.
func fetchAfterLogin(t *testing.T, am *alertmanagerAPI) ([]model.Alert, error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		alerts, err := am.Fetch(context.Background())
		if !errors.Is(err, ErrLoginPending) {
			return alerts, err
		}
		if time.Now().After(deadline) {
			t.Fatal("device login never finished")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestOIDCDeviceAuthFailsBeforeExpiry(t *testing.T) {
	auth := newOIDCDeviceAuthenticator("oidc-am", config.AuthConfig{
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

	auth := newOIDCDeviceAuthenticator("oidc-am", config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     server.URL,
		ClientID:      "foghorn-test",
		PersistTokens: testBoolPointer(false),
	}, server.Client())

	_, err := oidcTokenAfterLogin(t, auth)
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

	auth := newOIDCDeviceAuthenticator("oidc-am", config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     server.URL,
		ClientID:      "foghorn-test",
		PersistTokens: testBoolPointer(false),
	}, server.Client())

	_, err := oidcTokenAfterLogin(t, auth)
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

// F1: the poll engine bounds a fetch with the source's timeout (10s by
// default), but a device login waits on a person completing SSO and MFA. The
// login must run on its own lifetime, or it can never finish.
func TestOIDCLoginIsNotBoundByCallerDeadline(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/device":
			// Outlive any deadline the caller could reasonably impose.
			time.Sleep(50 * time.Millisecond)
			writeJSON(t, w, map[string]interface{}{
				"device_code":      "device-123",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://login.example.test/device",
				"expires_in":       600,
			})
		case "/token":
			writeJSON(t, w, map[string]interface{}{
				"access_token": "late-access",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	auth := newOIDCDeviceAuthenticatorWithStore("oidc-am", config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     server.URL,
		ClientID:      "foghorn-test",
		PersistTokens: testBoolPointer(false),
	}, server.Client(), newMemoryTokenStore())

	expiring, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := auth.Token(expiring); !errors.Is(err, ErrLoginPending) {
		t.Fatalf("Token() error = %v, want ErrLoginPending", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		short, cancelShort := context.WithTimeout(context.Background(), 5*time.Millisecond)
		token, err := auth.Token(short)
		cancelShort()
		if err == nil {
			if token.AccessToken != "late-access" {
				t.Fatalf("AccessToken = %q", token.AccessToken)
			}
			return
		}
		if !errors.Is(err, ErrLoginPending) {
			t.Fatalf("login failed under a short caller deadline: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("login never completed")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// F1: while a login is pending, further polls must join it. Minting a fresh
// device code per poll opened a browser tab every poll interval, each showing a
// different user code and only the newest one working.
func TestOIDCPendingLoginOpensOneBrowserTab(t *testing.T) {
	var deviceRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/device":
			deviceRequests.Add(1)
			writeJSON(t, w, map[string]interface{}{
				"device_code":      "device-123",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://login.example.test/device",
				"expires_in":       600,
				"interval":         1,
			})
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var opened atomic.Int32
	original := browserOpenURL
	browserOpenURL = func(string) error {
		opened.Add(1)
		return nil
	}
	defer func() { browserOpenURL = original }()

	auth := newOIDCDeviceAuthenticatorWithStore("oidc-am", config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     server.URL,
		ClientID:      "foghorn-test",
		PersistTokens: testBoolPointer(false),
	}, server.Client(), newMemoryTokenStore())

	for i := 0; i < 10; i++ {
		if _, err := auth.Token(context.Background()); !errors.Is(err, ErrLoginPending) {
			t.Fatalf("poll %d: error = %v, want ErrLoginPending", i+1, err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got := opened.Load(); got != 1 {
		t.Fatalf("opened %d browser tabs during one pending login, want 1", got)
	}
	if got := deviceRequests.Load(); got != 1 {
		t.Fatalf("started %d device authorizations during one pending login, want 1", got)
	}
}

// F3: the authenticator mutex used to be held across the whole interactive
// flow, so the settings view (GetOIDCSessions) and the log-out button
// (ForgetOIDCLogin) hung until the login finished.
func TestOIDCSessionQueriesStayResponsiveDuringLogin(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	var tokenPolls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/device":
			writeJSON(t, w, map[string]interface{}{
				"device_code":      "device-123",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://login.example.test/device",
				"expires_in":       600,
			})
		case "/token":
			tokenPolls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	auth := newOIDCDeviceAuthenticatorWithStore("oidc-am", config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     server.URL,
		ClientID:      "foghorn-test",
		PersistTokens: testBoolPointer(false),
	}, server.Client(), newMemoryTokenStore())

	if _, err := auth.Token(context.Background()); !errors.Is(err, ErrLoginPending) {
		t.Fatalf("Token() error = %v, want ErrLoginPending", err)
	}
	// Wait until the login is parked between token polls, which is exactly where
	// the old code sat holding the mutex.
	waitFor(t, func() bool { return tokenPolls.Load() > 0 }, "login never reached the token poll")
	if !auth.SessionInfo().LoginPending {
		t.Fatal("SessionInfo did not report the pending sign-in")
	}

	done := make(chan OIDCSessionInfo, 1)
	go func() {
		info := auth.SessionInfo()
		_ = auth.Forget()
		done <- info
	}()

	select {
	case info := <-done:
		if !info.LoginPending {
			t.Fatal("SessionInfo did not report the pending sign-in")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SessionInfo/Forget blocked on the in-flight login")
	}
}

// F3: logging out is also how a user abandons a login they no longer want, so
// it cancels the in-flight one and does not leave the retry backoff armed.
func TestOIDCForgetCancelsInFlightLogin(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	var deviceRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/device":
			deviceRequests.Add(1)
			writeJSON(t, w, map[string]interface{}{
				"device_code":      "device-123",
				"user_code":        "ABCD-EFGH",
				"verification_uri": "https://login.example.test/device",
				"expires_in":       600,
			})
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	auth := newOIDCDeviceAuthenticatorWithStore("oidc-am", config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     server.URL,
		ClientID:      "foghorn-test",
		PersistTokens: testBoolPointer(false),
	}, server.Client(), newMemoryTokenStore())

	if _, err := auth.Token(context.Background()); !errors.Is(err, ErrLoginPending) {
		t.Fatalf("Token() error = %v, want ErrLoginPending", err)
	}
	waitFor(t, func() bool { return deviceRequests.Load() > 0 }, "login never requested a device code")
	if err := auth.Forget(); err != nil {
		t.Fatalf("Forget() error: %v", err)
	}

	// A new sign-in must start immediately rather than waiting out the backoff
	// that a genuinely failed login would arm.
	deadline := time.Now().Add(5 * time.Second)
	for deviceRequests.Load() < 2 {
		if _, err := auth.Token(context.Background()); err != nil && !errors.Is(err, ErrLoginPending) {
			t.Fatalf("Token() after Forget: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("cancelled login did not release: device requests = %d", deviceRequests.Load())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// F2: manually configured endpoints bypassed discovery, and with it every
// transport check, while newFormRequest still sends client_id and
// client_secret to them in an HTTP Basic header.
func TestOIDCRejectsInsecureManualEndpoints(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	auth := newOIDCDeviceAuthenticatorWithStore("oidc-am", config.AuthConfig{
		Type:                   "oidc",
		Flow:                   "device",
		DeviceAuthorizationURL: "http://sso.example.test/device",
		TokenURL:               "http://sso.example.test/token",
		ClientID:               "foghorn-test",
		ClientSecret:           "super-secret",
		PersistTokens:          testBoolPointer(false),
	}, http.DefaultClient, newMemoryTokenStore())

	_, err := oidcTokenAfterLogin(t, auth)
	if err == nil {
		t.Fatal("expected cleartext manual endpoints to be rejected")
	}
	if !strings.Contains(err.Error(), "must be https") {
		t.Fatalf("error = %v, want an https requirement", err)
	}
}

// Loopback stays exempt: local development issuers commonly run without TLS.
func TestOIDCAllowsLoopbackManualEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://127.0.0.1:9000/device", "http://localhost:9000/device"} {
		if _, err := requireSecureEndpoint("device_authorization_url", endpoint); err != nil {
			t.Errorf("requireSecureEndpoint(%q) = %v, want allowed", endpoint, err)
		}
	}
	if _, err := requireSecureEndpoint("token_url", "https://sso.example.test/token"); err != nil {
		t.Errorf("https endpoint rejected: %v", err)
	}
}

// waitFor blocks until cond holds, failing the test if it never does.
func waitFor(t *testing.T, cond func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
