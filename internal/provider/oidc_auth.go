package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/browser"

	"foghorn/internal/config"
)

const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

type oidcDeviceAuthenticator struct {
	cfg    config.AuthConfig
	client *http.Client

	mu        sync.Mutex
	discovery *oidcDiscovery
	token     *oidcToken
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

func newOIDCDeviceAuthenticator(auth config.AuthConfig, client *http.Client) *oidcDeviceAuthenticator {
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
	return &oidcDeviceAuthenticator{cfg: auth, client: client}
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

func (a *oidcDeviceAuthenticator) Token(ctx context.Context) (*oidcToken, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token != nil && !a.token.expired() {
		return a.token, nil
	}
	if a.token != nil && a.token.RefreshToken != "" {
		if token, err := a.refresh(ctx, a.token.RefreshToken); err == nil {
			a.token = token
			return token, nil
		} else {
			log.Printf("oidc: refresh failed, starting device authorization: %v", err)
		}
	}

	discovery, err := a.discover(ctx)
	if err != nil {
		return nil, err
	}
	device, err := a.startDeviceAuthorization(ctx, discovery.DeviceAuthorizationEndpoint)
	if err != nil {
		return nil, err
	}
	a.promptUser(device)

	token, err := a.pollToken(ctx, discovery.TokenEndpoint, device)
	if err != nil {
		return nil, err
	}
	a.token = token
	return token, nil
}

func (a *oidcDeviceAuthenticator) discover(ctx context.Context) (*oidcDiscovery, error) {
	if a.discovery != nil {
		return a.discovery, nil
	}
	if a.cfg.DeviceAuthorizationURL != "" && a.cfg.TokenURL != "" {
		a.discovery = &oidcDiscovery{DeviceAuthorizationEndpoint: a.cfg.DeviceAuthorizationURL, TokenEndpoint: a.cfg.TokenURL}
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oidc discovery returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var discovery oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("decoding oidc discovery: %w", err)
	}
	if discovery.DeviceAuthorizationEndpoint == "" || discovery.TokenEndpoint == "" {
		return nil, errors.New("oidc discovery response missing device_authorization_endpoint or token_endpoint")
	}
	a.discovery = &discovery
	return a.discovery, nil
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("oidc device authorization returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var device oidcDeviceAuthorization
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
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
	if loginURL != "" && os.Getenv("FOGHORN_OIDC_SKIP_BROWSER") == "" {
		if err := browser.OpenURL(loginURL); err != nil {
			log.Printf("oidc: open browser failed: %v", err)
		}
	}
	if device.VerificationURIComplete != "" {
		log.Printf("oidc: opened browser for device login: %s", device.VerificationURIComplete)
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

func (a *oidcDeviceAuthenticator) refresh(ctx context.Context, refreshToken string) (*oidcToken, error) {
	discovery, err := a.discover(ctx)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", a.cfg.ClientID)
	form.Set("refresh_token", refreshToken)
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
	return token, nil
}

type oidcTokenRetry struct {
	SlowDown bool
}

func decodeDeviceTokenResponse(resp *http.Response) (*oidcToken, *oidcTokenRetry, error) {
	defer resp.Body.Close()
	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
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
		return nil, nil, fmt.Errorf("oidc token endpoint returned HTTP %d: %s", resp.StatusCode, errorCode)
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
		return false
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
