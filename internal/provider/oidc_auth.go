package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/browser"

	"foghorn/internal/config"
	"foghorn/internal/keyring"
)

const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// browserOpenURL launches the OS's default browser; a variable so tests can
// verify what promptUser hands it without actually opening a browser.
var browserOpenURL = browser.OpenURL

// ErrLoginPending reports that an interactive device login is running in the
// background and has not finished yet. A caller on a poll deadline must fail
// fast with this rather than wait: the login waits on a human, so blocking
// would fail the poll anyway and abandon a login that is still completable.
var ErrLoginPending = errors.New("browser sign-in is pending")

const (
	// oidcNetworkTimeout bounds the machine-speed steps of a login (discovery,
	// device authorization, refresh). Waiting for the user is bounded separately
	// by the device code's own expiry.
	oidcNetworkTimeout = 30 * time.Second

	// oidcLoginRetryBackoff throttles device logins after a failed attempt, so an
	// app nobody is watching cannot reopen a browser tab every poll interval.
	oidcLoginRetryBackoff = 5 * time.Minute
)

type oidcDeviceAuthenticator struct {
	source             string
	cfg                config.AuthConfig
	client             *http.Client
	store              keyring.Store
	account            string
	legacyAccount      string
	identity           string
	persistenceEnabled bool
	storeSupported     bool
	storageBackend     string

	// discovery is resolved outside a.mu: it is reached from a login running in
	// the background, which must not block session queries.
	discoveryMu sync.Mutex
	discovery   *oidcDiscovery

	mu            sync.Mutex
	token         *oidcToken
	tokenDirty    bool
	loadComplete  bool
	persisted     bool
	legacyPending bool
	storageError  string

	// flight is the one in-progress token acquisition. Serializing acquisitions
	// keeps exactly one browser tab open per device code, and stops two
	// concurrent refreshes from each rotating the other's refresh token.
	flight       *tokenFlight
	retryAfter   time.Time
	lastLoginErr error
	// forgetGen invalidates a flight that finishes after the user logged out, so
	// a login completing moments later cannot resurrect the session.
	forgetGen   uint64
	loginCtx    context.Context
	loginCancel context.CancelFunc
}

// tokenFlight is a single in-progress acquisition of a usable token, shared by
// every caller that arrives while it runs.
type tokenFlight struct {
	done        chan struct{}
	interactive bool
	token       *oidcToken
	err         error
}

type oidcDiscovery struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

type oidcDeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type oidcToken struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	obtainedAt   time.Time
}

func newOIDCDeviceAuthenticator(source string, auth config.AuthConfig, client *http.Client) *oidcDeviceAuthenticator {
	if auth.PersistTokens != nil && *auth.PersistTokens && !keyring.Supported() {
		log.Printf("oidc: source %q requested persistent tokens, but secure token storage is unavailable on this platform; using memory-only credentials", source)
	}
	a := newOIDCDeviceAuthenticatorWithStore(source, auth, client, keyring.NewOIDCStore())
	if a != nil {
		a.persistenceEnabled = oidcPersistenceEnabled(auth)
		a.storeSupported = keyring.Supported()
		a.storageBackend = keyring.BackendName()
	}
	return a
}

func newOIDCDeviceAuthenticatorWithStore(source string, auth config.AuthConfig, client *http.Client, store keyring.Store) *oidcDeviceAuthenticator {
	authType := strings.ToLower(strings.TrimSpace(auth.Type))
	flow := strings.ToLower(strings.TrimSpace(auth.Flow))
	if authType != "oidc" && authType != "oidc_device" {
		return nil
	}
	if flow == "" {
		flow = "device"
	}
	if flow != "device" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	// A device login outlives the request that starts it, so it runs on a
	// context the authenticator owns rather than the caller's poll deadline.
	loginCtx, loginCancel := context.WithCancel(context.Background())
	return &oidcDeviceAuthenticator{
		source:             source,
		cfg:                auth,
		client:             client,
		store:              store,
		account:            OIDCTokenAccount(source),
		legacyAccount:      OIDCLegacyTokenAccount(source, auth),
		identity:           OIDCTokenIdentity(source, auth),
		persistenceEnabled: auth.PersistTokens == nil || *auth.PersistTokens,
		storeSupported:     true,
		storageBackend:     keyring.BackendName(),
		loginCtx:           loginCtx,
		loginCancel:        loginCancel,
	}
}

func (a *oidcDeviceAuthenticator) Apply(ctx context.Context, req *http.Request) error {
	tok, err := a.Token(ctx)
	if err != nil {
		return err
	}
	bearer := tok.AccessToken
	if a.cfg.UseIDToken {
		bearer = tok.IDToken
	}
	if strings.TrimSpace(bearer) == "" {
		return errors.New("oidc: token response did not contain the configured token")
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	return nil
}

func (a *oidcDeviceAuthenticator) SessionInfo() OIDCSessionInfo {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.loadPersistedTokenLocked()
	return OIDCSessionInfo{
		Source:             a.source,
		Configured:         true,
		Active:             a.token != nil,
		Saved:              a.persisted,
		LoginPending:       a.flight != nil && a.flight.interactive,
		PersistenceEnabled: a.persistenceEnabled,
		StorageBackend:     a.storageBackend,
		StorageError:       a.storageError,
	}
}

// expireAccessToken forces the next Apply to refresh or reauthenticate. It is
// used after a resource server rejects an otherwise unexpired bearer token.
func (a *oidcDeviceAuthenticator) expireAccessToken() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != nil {
		a.token.ExpiresIn = 0
	}
}

func (a *oidcDeviceAuthenticator) Forget() error {
	a.mu.Lock()
	a.token = nil
	a.tokenDirty = false
	a.loadComplete = true
	// Logging out is also how a user cancels a login they no longer want, and it
	// clears the backoff so signing in again is immediate rather than throttled.
	a.retryAfter = time.Time{}
	a.lastLoginErr = nil
	a.forgetGen++
	cancel := a.loginCancel
	a.loginCtx, a.loginCancel = context.WithCancel(context.Background())
	err := a.deletePersistedTokenLocked()
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return err
}

// Token returns a usable token, starting a refresh or a device login when there
// is none. Acquisitions are serialized into a single flight so that concurrent
// callers share one login rather than each minting a device code of its own.
func (a *oidcDeviceAuthenticator) Token(ctx context.Context) (*oidcToken, error) {
	a.mu.Lock()

	a.loadPersistedTokenLocked()
	if a.token != nil && !a.token.expired() {
		a.savePersistedTokenLocked()
		token := a.token
		a.mu.Unlock()
		return token, nil
	}

	if flight := a.flight; flight != nil {
		a.mu.Unlock()
		return a.awaitFlight(ctx, flight)
	}

	interactive := a.token == nil || a.token.RefreshToken == ""
	if interactive && time.Now().Before(a.retryAfter) {
		err := fmt.Errorf("oidc: source %q: sign-in failed and will be retried in %s: %w",
			a.source, time.Until(a.retryAfter).Round(time.Second), a.lastLoginErr)
		a.mu.Unlock()
		return nil, err
	}

	flight := &tokenFlight{done: make(chan struct{}), interactive: interactive}
	a.flight = flight
	previous := a.token
	loginCtx := a.loginCtx
	generation := a.forgetGen
	a.mu.Unlock()

	go a.runFlight(loginCtx, flight, previous, generation)
	return a.awaitFlight(ctx, flight)
}

// awaitFlight blocks only for acquisitions that run at machine speed. An
// interactive login waits on a person, so its callers are told the login is
// pending and left to retry on their own schedule.
func (a *oidcDeviceAuthenticator) awaitFlight(ctx context.Context, flight *tokenFlight) (*oidcToken, error) {
	if flight.interactive {
		select {
		case <-flight.done:
			return flight.token, flight.err
		default:
			return nil, fmt.Errorf("oidc: source %q: %w", a.source, ErrLoginPending)
		}
	}
	select {
	case <-flight.done:
		return flight.token, flight.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// runFlight performs one acquisition off a.mu, so that session queries and
// logout stay responsive while a login or refresh is in progress.
func (a *oidcDeviceAuthenticator) runFlight(ctx context.Context, flight *tokenFlight, previous *oidcToken, generation uint64) {
	var token *oidcToken
	var err error
	if flight.interactive {
		token, err = a.performDeviceLogin(ctx)
	} else {
		token, err = a.performRefresh(ctx, previous)
	}

	a.mu.Lock()
	switch {
	case generation != a.forgetGen:
		// The user logged out while this ran; its result is no longer wanted.
		if err == nil {
			err = fmt.Errorf("oidc: source %q: sign-in was cancelled", a.source)
			token = nil
		}
	case err == nil:
		a.token = token
		a.tokenDirty = true
		a.savePersistedTokenLocked()
		a.retryAfter = time.Time{}
		a.lastLoginErr = nil
	case flight.interactive && !errors.Is(err, context.Canceled):
		a.retryAfter = time.Now().Add(oidcLoginRetryBackoff)
		a.lastLoginErr = err
	}
	a.flight = nil
	a.mu.Unlock()

	if err == nil {
		logOIDCTokenAuthorizationClaims(token)
	}
	flight.token, flight.err = token, err
	close(flight.done)
}

// performRefresh exchanges the saved refresh token. A refresh rejected as
// invalid (4xx) means the saved login is dead: it is discarded so the next call
// starts a device login, while a transient failure keeps it for a later retry.
func (a *oidcDeviceAuthenticator) performRefresh(ctx context.Context, previous *oidcToken) (*oidcToken, error) {
	refreshCtx, cancel := context.WithTimeout(ctx, oidcNetworkTimeout)
	defer cancel()

	token, err := a.refresh(refreshCtx, previous)
	if err == nil {
		return token, nil
	}
	if !isNonRetryableOIDCRefreshError(err) {
		return nil, fmt.Errorf("oidc: refreshing saved login for source %q: %w", a.source, err)
	}

	log.Printf("oidc: saved login for source %q is no longer valid; a new sign-in is required", a.source)
	a.mu.Lock()
	a.token = nil
	a.tokenDirty = false
	if deleteErr := a.deletePersistedTokenLocked(); deleteErr != nil {
		log.Printf("oidc: source %q could not remove the invalid saved login: %v", a.source, deleteErr)
	}
	a.mu.Unlock()
	return nil, fmt.Errorf("oidc: saved login for source %q is no longer valid; signing in again: %w", a.source, err)
}

// performDeviceLogin runs the RFC 8628 device flow. Its network steps are
// bounded by oidcNetworkTimeout; the wait for the user is bounded only by the
// device code's expiry, which is what makes a login survive a source timeout
// far shorter than a person needs to complete SSO and MFA.
func (a *oidcDeviceAuthenticator) performDeviceLogin(ctx context.Context) (*oidcToken, error) {
	setupCtx, cancel := context.WithTimeout(ctx, oidcNetworkTimeout)
	defer cancel()

	discovery, err := a.discover(setupCtx)
	if err != nil {
		return nil, err
	}
	device, err := a.startDeviceAuthorization(setupCtx, discovery.DeviceAuthorizationEndpoint)
	if err != nil {
		return nil, err
	}
	a.promptUser(device)

	return a.pollToken(ctx, discovery.TokenEndpoint, device)
}

func (a *oidcDeviceAuthenticator) discover(ctx context.Context) (*oidcDiscovery, error) {
	a.discoveryMu.Lock()
	defer a.discoveryMu.Unlock()

	if a.discovery != nil {
		return a.discovery, nil
	}
	if a.cfg.DeviceAuthorizationURL != "" && a.cfg.TokenURL != "" {
		// Manually configured endpoints skip discovery, but newFormRequest still
		// sends client_id and client_secret to them via HTTP Basic auth, so they
		// need the same transport guarantee. There is no discovery document to
		// distrust here, hence no issuer-origin pin.
		deviceEndpoint, err := requireSecureEndpoint("device_authorization_url", a.cfg.DeviceAuthorizationURL)
		if err != nil {
			return nil, err
		}
		tokenEndpoint, err := requireSecureEndpoint("token_url", a.cfg.TokenURL)
		if err != nil {
			return nil, err
		}
		a.discovery = &oidcDiscovery{DeviceAuthorizationEndpoint: deviceEndpoint, TokenEndpoint: tokenEndpoint}
		return a.discovery, nil
	}
	issuer := strings.TrimRight(strings.TrimSpace(a.cfg.IssuerURL), "/")
	if issuer == "" {
		return nil, errors.New("oidc: issuer_url is required unless device_authorization_url and token_url are configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery returned HTTP %d: %s", resp.StatusCode, errorBody(resp))
	}
	var discovery oidcDiscovery
	if err := decodeJSONResponse(resp, &discovery); err != nil {
		return nil, fmt.Errorf("decoding oidc discovery: %w", err)
	}
	if discovery.DeviceAuthorizationEndpoint == "" || discovery.TokenEndpoint == "" {
		return nil, errors.New("oidc discovery response missing device_authorization_endpoint or token_endpoint")
	}
	// The discovery document is fetched from the issuer but its contents are
	// still attacker-controlled if the issuer is compromised (or MITM'd, when
	// issuer_url is http://): newFormRequest sends client_id+client_secret via
	// HTTP Basic auth to whatever token_endpoint/device_authorization_endpoint
	// this returns. Require https and pin both endpoints to the issuer's own
	// origin so a hostile discovery response cannot redirect the client
	// credentials to an attacker-controlled host.
	deviceEndpoint, err := validateDiscoveredEndpoint("device_authorization_endpoint", issuer, discovery.DeviceAuthorizationEndpoint)
	if err != nil {
		return nil, err
	}
	tokenEndpoint, err := validateDiscoveredEndpoint("token_endpoint", issuer, discovery.TokenEndpoint)
	if err != nil {
		return nil, err
	}
	discovery.DeviceAuthorizationEndpoint = deviceEndpoint
	discovery.TokenEndpoint = tokenEndpoint
	a.discovery = &discovery
	return a.discovery, nil
}

// validateDiscoveredEndpoint checks an endpoint URL returned by OIDC discovery
// before it is used to send requests (and, for the token endpoint, client
// credentials). It must share the issuer's origin, and be https unless it is a
// loopback address (local dev/test issuers commonly run without TLS).
func validateDiscoveredEndpoint(kind, issuer, endpoint string) (string, error) {
	trimmed, err := requireSecureEndpoint(kind, endpoint)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("oidc discovery: %s %q is not a valid URL: %w", kind, trimmed, err)
	}
	issuerURL, err := url.Parse(issuer)
	if err != nil || issuerURL.Host == "" {
		return "", fmt.Errorf("oidc discovery: cannot validate %s against issuer %q", kind, issuer)
	}
	if !strings.EqualFold(issuerURL.Host, parsed.Host) {
		return "", fmt.Errorf("oidc discovery: %s %q is not on the issuer's origin (%s)", kind, trimmed, issuerURL.Host)
	}
	return trimmed, nil
}

// requireSecureEndpoint rejects an OIDC endpoint that would carry client
// credentials in cleartext. Loopback is exempt because local development
// issuers commonly run without TLS.
func requireSecureEndpoint(kind, endpoint string) (string, error) {
	trimmed := strings.TrimSpace(endpoint)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("oidc: %s %q is not a valid URL: %w", kind, trimmed, err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") && !isLoopbackHost(parsed.Hostname()) {
		return "", fmt.Errorf("oidc: %s %q must be https: client credentials and tokens would otherwise be sent in cleartext", kind, trimmed)
	}
	return trimmed, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

func (a *oidcDeviceAuthenticator) startDeviceAuthorization(ctx context.Context, endpoint string) (*oidcDeviceAuthorization, error) {
	if strings.TrimSpace(a.cfg.ClientID) == "" {
		return nil, errors.New("oidc: client_id is required")
	}
	form := url.Values{}
	form.Set("client_id", a.cfg.ClientID)
	if len(a.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(a.cfg.Scopes, " "))
	}
	req, err := newFormRequest(ctx, endpoint, form, a.cfg.ClientID, a.cfg.ClientSecret)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc device authorization: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc device authorization returned HTTP %d: %s", resp.StatusCode, errorBody(resp))
	}
	var device oidcDeviceAuthorization
	if err := decodeJSONResponse(resp, &device); err != nil {
		return nil, fmt.Errorf("decoding oidc device authorization: %w", err)
	}
	if device.DeviceCode == "" || (device.VerificationURI == "" && device.VerificationURIComplete == "") {
		return nil, errors.New("oidc device authorization response missing device_code or verification URI")
	}
	return &device, nil
}

func (a *oidcDeviceAuthenticator) promptUser(device *oidcDeviceAuthorization) {
	loginURL := device.VerificationURIComplete
	if loginURL == "" {
		loginURL = device.VerificationURI
	}
	// verification_uri(_complete) comes from the token endpoint's response, so a
	// compromised/hostile endpoint could put a file://, smb:// or other
	// locally-reachable URL here instead of a login page. Only ever hand
	// http/https to the OS's browser opener.
	safeLoginURL := sanitizeRemoteURL(loginURL)
	if safeLoginURL == "" && loginURL != "" {
		log.Printf("oidc: refusing to open non-http(s) verification URL %q", loginURL)
	}
	if safeLoginURL != "" && os.Getenv("FOGHORN_OIDC_SKIP_BROWSER") == "" {
		if err := browserOpenURL(safeLoginURL); err != nil {
			log.Printf("oidc: open browser failed: %v", err)
		}
	}
	if device.VerificationURIComplete != "" {
		log.Printf("oidc: device login link: %s", device.VerificationURIComplete)
		return
	}
	log.Printf("oidc: open %s and enter user code %s", device.VerificationURI, device.UserCode)
}

func (a *oidcDeviceAuthenticator) pollToken(ctx context.Context, endpoint string, device *oidcDeviceAuthorization) (*oidcToken, error) {
	interval := time.Duration(device.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expiresAt := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	if device.ExpiresIn <= 0 {
		expiresAt = time.Now().Add(10 * time.Minute)
	}
	for {
		if time.Now().After(expiresAt) {
			return nil, errors.New("oidc device authorization expired before login completed")
		}
		form := url.Values{}
		form.Set("grant_type", deviceCodeGrantType)
		form.Set("client_id", a.cfg.ClientID)
		form.Set("device_code", device.DeviceCode)
		req, err := newFormRequest(ctx, endpoint, form, a.cfg.ClientID, a.cfg.ClientSecret)
		if err != nil {
			return nil, err
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("oidc token poll: %w", err)
		}
		token, retry, err := decodeDeviceTokenResponse(resp)
		if err != nil {
			return nil, err
		}
		if token != nil {
			return token, nil
		}
		if retry != nil && retry.SlowDown {
			interval += 5 * time.Second
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (a *oidcDeviceAuthenticator) refresh(ctx context.Context, previous *oidcToken) (*oidcToken, error) {
	discovery, err := a.discover(ctx)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.cfg.ClientID)
	form.Set("refresh_token", previous.RefreshToken)
	req, err := newFormRequest(ctx, discovery.TokenEndpoint, form, a.cfg.ClientID, a.cfg.ClientSecret)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc refresh: %w", err)
	}
	token, _, err := decodeDeviceTokenResponse(resp)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("oidc refresh did not return a token")
	}
	if token.RefreshToken == "" {
		token.RefreshToken = previous.RefreshToken
	}
	if token.IDToken == "" {
		token.IDToken = previous.IDToken
	}
	configuredToken := token.AccessToken
	if a.cfg.UseIDToken {
		configuredToken = token.IDToken
	}
	if strings.TrimSpace(configuredToken) == "" {
		return nil, errors.New("oidc refresh did not contain the configured token")
	}
	return token, nil
}

type oidcTokenRetry struct {
	SlowDown bool
}

type oidcTokenEndpointError struct {
	Status int
	Code   string
}

func (e *oidcTokenEndpointError) Error() string {
	return fmt.Sprintf("oidc token endpoint returned HTTP %d: %s", e.Status, e.Code)
}

func isNonRetryableOIDCRefreshError(err error) bool {
	var endpointErr *oidcTokenEndpointError
	if !errors.As(err, &endpointErr) {
		return false
	}
	return endpointErr.Status >= 400 && endpointErr.Status < 500
}

func decodeDeviceTokenResponse(resp *http.Response) (*oidcToken, *oidcTokenRetry, error) {
	defer resp.Body.Close()
	var raw map[string]interface{}
	if err := decodeJSONResponse(resp, &raw); err != nil {
		return nil, nil, fmt.Errorf("decoding oidc token response: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		token := &oidcToken{
			AccessToken:  stringValue(raw["access_token"]),
			IDToken:      stringValue(raw["id_token"]),
			TokenType:    stringValue(raw["token_type"]),
			ExpiresIn:    intValue(raw["expires_in"]),
			RefreshToken: stringValue(raw["refresh_token"]),
			obtainedAt:   time.Now(),
		}
		if token.TokenType == "" {
			token.TokenType = "Bearer"
		}
		return token, nil, nil
	}
	errorCode := stringValue(raw["error"])
	switch errorCode {
	case "authorization_pending":
		return nil, &oidcTokenRetry{}, nil
	case "slow_down":
		return nil, &oidcTokenRetry{SlowDown: true}, nil
	default:
		return nil, nil, &oidcTokenEndpointError{Status: resp.StatusCode, Code: errorCode}
	}
}

func newFormRequest(ctx context.Context, endpoint string, form url.Values, clientID, clientSecret string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	return req, nil
}

func (t *oidcToken) expired() bool {
	if t == nil {
		return true
	}
	if t.ExpiresIn <= 0 {
		return true
	}
	return time.Now().After(t.obtainedAt.Add(time.Duration(t.ExpiresIn)*time.Second - 30*time.Second))
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intValue(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// logOIDCTokenAuthorizationClaims logs only claims useful for diagnosing
// gateway authorization. It deliberately excludes identity claims and never
// logs the encoded token itself. Both tokens are inspected because some OIDC
// client mappings accidentally add groups to the ID token but not the access
// token that Foghorn sends to Alertmanager.
func logOIDCTokenAuthorizationClaims(token *oidcToken) {
	if !HTTPDebugEnabled() || token == nil {
		return
	}
	for _, candidate := range []struct {
		name string
		raw  string
	}{
		{name: "access", raw: token.AccessToken},
		{name: "id", raw: token.IDToken},
	} {
		if strings.TrimSpace(candidate.raw) == "" {
			continue
		}
		claims, err := decodeOIDCAuthorizationClaims(candidate.raw)
		if err != nil {
			log.Printf("oidc: unable to decode %s token authorization claims: %v", candidate.name, err)
			continue
		}
		encoded, err := json.Marshal(claims)
		if err != nil {
			log.Printf("oidc: unable to format %s token authorization claims: %v", candidate.name, err)
			continue
		}
		log.Printf("oidc: %s token authorization claims=%s", candidate.name, encoded)
	}
}

func decodeOIDCAuthorizationClaims(rawToken string) (map[string]interface{}, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected JWT with 3 segments, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var all map[string]interface{}
	if err := json.Unmarshal(payload, &all); err != nil {
		return nil, fmt.Errorf("decoding JWT claims: %w", err)
	}

	filtered := make(map[string]interface{})
	for name, value := range all {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "group") || strings.Contains(lower, "role") || isOIDCAuthorizationContextClaim(lower) {
			filtered[name] = value
		}
	}
	return filtered, nil
}

func isOIDCAuthorizationContextClaim(name string) bool {
	switch name {
	case "iss", "aud", "azp", "client_id", "scope", "scp", "typ", "realm_access", "resource_access":
		return true
	default:
		return false
	}
}
