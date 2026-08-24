package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/keyring"
)

type memoryTokenStore struct {
	items       map[string][]byte
	getErr      error
	getErrors   map[string]error
	setFailures int
	deleteErr   error
	setCalls    int
	deleteCalls int
	maxSecret   int
}

func (s *memoryTokenStore) MaxSecretSize() int { return s.maxSecret }

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{
		items:     make(map[string][]byte),
		getErrors: make(map[string]error),
	}
}

func (s *memoryTokenStore) Get(account string) ([]byte, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if err := s.getErrors[account]; err != nil {
		return nil, err
	}
	value, ok := s.items[account]
	if !ok {
		return nil, keyring.ErrNotFound
	}
	return slices.Clone(value), nil
}

func (s *memoryTokenStore) Set(account string, secret []byte) error {
	s.setCalls++
	if s.setFailures > 0 {
		s.setFailures--
		return errors.New("test keyring unavailable")
	}
	if s.maxSecret > 0 && len(secret) > s.maxSecret {
		return errors.New("test keyring value too big")
	}
	s.items[account] = slices.Clone(secret)
	return nil
}

func (s *memoryTokenStore) Delete(account string) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.items, account)
	return nil
}

func persistentTestAuth(issuer string) config.AuthConfig {
	return config.AuthConfig{
		Type:          "oidc",
		Flow:          "device",
		IssuerURL:     issuer,
		ClientID:      "foghorn-test",
		Scopes:        []string{"openid", "offline_access"},
		PersistTokens: testBoolPointer(true),
	}
}

func saveTestToken(t *testing.T, store *memoryTokenStore, auth *oidcDeviceAuthenticator, token *oidcToken) {
	t.Helper()
	encoded, err := marshalPersistedOIDCToken(token, auth.identity)
	if err != nil {
		t.Fatal(err)
	}
	store.items[auth.account] = encoded
}

// oidcTokenAfterLogin drives Token the way the poll engine does. A device login
// runs in the background and its callers are told it is pending, so they retry
// on their own schedule instead of holding a request open across a human's
// sign-in. It returns the first result that is not "pending".
func oidcTokenAfterLogin(t *testing.T, auth *oidcDeviceAuthenticator) (*oidcToken, error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		token, err := auth.Token(context.Background())
		if !errors.Is(err, ErrLoginPending) {
			return token, err
		}
		if time.Now().After(deadline) {
			t.Fatal("device login never finished")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func tokenWhenSignedIn(t *testing.T, auth *oidcDeviceAuthenticator) *oidcToken {
	t.Helper()
	token, err := oidcTokenAfterLogin(t, auth)
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	return token
}

func TestOIDCTokenIdentityIsStableAndConfigurationScoped(t *testing.T) {
	auth := persistentTestAuth("HTTPS://Login.Example.Test/realm/")
	auth.Scopes = []string{"offline_access", "openid", "openid"}
	first := OIDCTokenIdentity("production", auth)

	auth.IssuerURL = "https://login.example.test/realm"
	auth.Scopes = []string{"openid", "offline_access"}
	if second := OIDCTokenIdentity("production", auth); second != first {
		t.Fatalf("equivalent identities produced different fingerprints: %q != %q", first, second)
	}
	if changed := OIDCTokenIdentity("staging", auth); changed == first {
		t.Fatal("source name must isolate saved credentials")
	}
	auth.UseIDToken = true
	if changed := OIDCTokenIdentity("production", auth); changed == first {
		t.Fatal("use_id_token must change the login identity")
	}
}

// The credential-store slot is keyed by source name alone, so editing a
// source's login configuration reuses its item instead of stranding one the
// user can neither list nor clear.
func TestOIDCTokenAccountDependsOnlyOnSourceName(t *testing.T) {
	auth := persistentTestAuth("https://login.example.test")
	account := OIDCTokenAccount("production")

	auth.ClientID = "rotated-client"
	auth.Scopes = append(auth.Scopes, "groups")
	auth.UseIDToken = true
	if changed := OIDCTokenAccount("production"); changed != account {
		t.Fatalf("login configuration changed the account: %q != %q", changed, account)
	}
	if OIDCTokenAccount("staging") == account {
		t.Fatal("different sources must not share a credential-store item")
	}
	if OIDCLegacyTokenAccount("production", auth) == account {
		t.Fatal("legacy account must be distinct from the current one")
	}
}

// A token minted for one login configuration must never be presented for
// another: the fingerprint check fails closed.
func TestOIDCRejectsSavedTokenFromDifferentIdentity(t *testing.T) {
	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)
	encoded, err := marshalPersistedOIDCToken(&oidcToken{
		AccessToken: "other-identity-access",
		ExpiresIn:   3600,
		obtainedAt:  time.Now(),
	}, "fingerprint-of-a-different-login")
	if err != nil {
		t.Fatal(err)
	}
	store.items[auth.account] = encoded

	if info := auth.SessionInfo(); info.Active || info.Saved {
		t.Fatalf("token from another identity was adopted: %#v", info)
	}
}

// A login saved under the pre-v2 layout must survive the upgrade: it is moved
// into the per-source slot and the old item is removed once the copy is safe.
func TestOIDCMigratesLegacyTokenAndRemovesOldItem(t *testing.T) {
	store := newMemoryTokenStore()
	authConfig := persistentTestAuth("https://login.example.test")
	auth := newOIDCDeviceAuthenticatorWithStore("production", authConfig, http.DefaultClient, store)

	legacy, err := json.Marshal(map[string]interface{}{
		"version":       1,
		"access_token":  "legacy-access",
		"refresh_token": "legacy-refresh",
		"expires_in":    3600,
		"obtained_at":   time.Now().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	store.items[auth.legacyAccount] = legacy

	token, err := auth.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if token.AccessToken != "legacy-access" {
		t.Fatalf("AccessToken = %q, want the migrated token", token.AccessToken)
	}
	if _, ok := store.items[auth.legacyAccount]; ok {
		t.Fatal("legacy credential-store item was left behind")
	}
	migrated, err := unmarshalPersistedOIDCToken(store.items[auth.account], auth.identity)
	if err != nil {
		t.Fatalf("migrated token: %v", err)
	}
	if migrated.RefreshToken != "legacy-refresh" {
		t.Fatalf("migrated RefreshToken = %q", migrated.RefreshToken)
	}
}

func TestOIDCLegacyKeyringReadFailureIsReportedAndRetried(t *testing.T) {
	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)
	store.getErrors[auth.legacyAccount] = errors.New("test legacy keyring locked")

	if info := auth.SessionInfo(); info.StorageError == "" || info.Saved {
		t.Fatalf("unexpected session state after legacy read failure: %#v", info)
	}
	delete(store.getErrors, auth.legacyAccount)
	if info := auth.SessionInfo(); info.StorageError != "" || info.Saved {
		t.Fatalf("legacy read was not retried after recovery: %#v", info)
	}
}

func TestOIDCCorruptLegacyTokenIsReportedAsSaved(t *testing.T) {
	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)
	store.items[auth.legacyAccount] = []byte("not-json")

	info := auth.SessionInfo()
	if info.StorageError == "" || !info.Saved || info.Active {
		t.Fatalf("unexpected session state for corrupt legacy token: %#v", info)
	}
}

// Windows Credential Manager caps a credential blob at 2560 bytes, which real
// JWTs exceed. The save drops the tokens that are not needed to restore the
// login rather than failing and silently degrading to memory-only credentials.
func TestOIDCPersistShrinksPayloadToFitCredentialStore(t *testing.T) {
	large := strings.Repeat("j", 1200)
	token := &oidcToken{
		AccessToken:  "access-" + large,
		IDToken:      "id-" + large,
		RefreshToken: "refresh-" + large,
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		obtainedAt:   time.Now(),
	}

	encoded, err := marshalPersistedOIDCTokenWithin(token, "identity", false, 2560)
	if err != nil {
		t.Fatalf("marshalPersistedOIDCTokenWithin() error: %v", err)
	}
	if len(encoded) > 2560 {
		t.Fatalf("payload is %d bytes, above the 2560 byte limit", len(encoded))
	}
	stored, err := unmarshalPersistedOIDCToken(encoded, "identity")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshToken != token.RefreshToken {
		t.Fatal("the refresh token must always survive: it is what restores the login")
	}
	if stored.IDToken != "" {
		t.Fatal("the unused ID token should be dropped first")
	}
	if stored.AccessToken != "" {
		t.Fatal("the access token should be dropped once the ID token is not enough")
	}
	if !stored.expired() {
		t.Fatal("a payload without the configured token must force a refresh on restore")
	}

	if _, err := marshalPersistedOIDCTokenWithin(token, "identity", false, 64); err == nil {
		t.Fatal("expected an error when even a refresh-only payload does not fit")
	}
}

// With use_id_token, the ID token is the credential the source sends, so the
// access token is the one to drop first.
func TestOIDCPersistKeepsConfiguredTokenWhenShrinking(t *testing.T) {
	large := strings.Repeat("j", 1200)
	token := &oidcToken{
		AccessToken:  "access-" + large,
		IDToken:      "id-" + large,
		RefreshToken: "refresh-short",
		ExpiresIn:    3600,
		obtainedAt:   time.Now(),
	}

	encoded, err := marshalPersistedOIDCTokenWithin(token, "identity", true, 2560)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := unmarshalPersistedOIDCToken(encoded, "identity")
	if err != nil {
		t.Fatal(err)
	}
	if stored.IDToken != token.IDToken {
		t.Fatalf("configured ID token was dropped before the access token")
	}
	if stored.AccessToken != "" {
		t.Fatal("access token should have been dropped to fit")
	}
	if stored.expired() {
		t.Fatal("the configured token is still present, so the login must stay usable")
	}
}

func TestOIDCRestoresValidTokenWithoutNetwork(t *testing.T) {
	store := newMemoryTokenStore()
	authConfig := persistentTestAuth("https://login.example.test")
	auth := newOIDCDeviceAuthenticatorWithStore("production", authConfig, http.DefaultClient, store)
	saveTestToken(t, store, auth, &oidcToken{
		AccessToken:  "saved-access",
		RefreshToken: "saved-refresh",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		obtainedAt:   time.Now(),
	})

	token, err := auth.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if token.AccessToken != "saved-access" {
		t.Fatalf("AccessToken = %q, want saved token", token.AccessToken)
	}
	if info := auth.SessionInfo(); !info.Active || !info.Saved || info.StorageError != "" || info.StorageBackend != keyring.BackendName() {
		t.Fatalf("unexpected session state: %#v", info)
	}
}

func TestOIDCRefreshRotatesAndPersistsRefreshToken(t *testing.T) {
	var refreshBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/token":
			refreshBody = readFormBody(t, r)
			writeJSON(t, w, map[string]interface{}{
				"access_token":  "rotated-access",
				"refresh_token": "rotated-refresh",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth(server.URL), server.Client(), store)
	saveTestToken(t, store, auth, &oidcToken{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresIn:    60,
		obtainedAt:   time.Now().Add(-time.Hour),
	})

	token, err := auth.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if token.RefreshToken != "rotated-refresh" {
		t.Fatalf("RefreshToken = %q, want rotated token", token.RefreshToken)
	}
	if !strings.Contains(refreshBody, "refresh_token=old-refresh") {
		t.Fatalf("refresh request did not use saved refresh token: %s", refreshBody)
	}
	stored, err := unmarshalPersistedOIDCToken(store.items[auth.account], auth.identity)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AccessToken != "rotated-access" || stored.RefreshToken != "rotated-refresh" {
		t.Fatalf("rotated token was not persisted: %#v", stored)
	}
}

func TestOIDCRefreshPreservesRefreshTokenWhenResponseOmitsIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/token":
			writeJSON(t, w, map[string]interface{}{"access_token": "new-access", "expires_in": 3600})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth(server.URL), server.Client(), store)
	saveTestToken(t, store, auth, &oidcToken{
		RefreshToken: "keep-refresh",
		ExpiresIn:    60,
		obtainedAt:   time.Now().Add(-time.Hour),
	})

	token, err := auth.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken != "keep-refresh" {
		t.Fatalf("RefreshToken = %q, want preserved token", token.RefreshToken)
	}
}

func TestOIDCRefreshPreservesIDTokenAndUnknownExpiryForcesRefresh(t *testing.T) {
	refreshRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/token":
			refreshRequests++
			writeJSON(t, w, map[string]interface{}{"access_token": "new-access"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryTokenStore()
	authConfig := persistentTestAuth(server.URL)
	authConfig.UseIDToken = true
	auth := newOIDCDeviceAuthenticatorWithStore("production", authConfig, server.Client(), store)
	saveTestToken(t, store, auth, &oidcToken{
		IDToken:      "saved-id",
		RefreshToken: "saved-refresh",
		ExpiresIn:    0,
		obtainedAt:   time.Now(),
	})

	for i := 0; i < 2; i++ {
		token, err := auth.Token(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if token.IDToken != "saved-id" {
			t.Fatalf("IDToken = %q, want preserved ID token", token.IDToken)
		}
	}
	if refreshRequests != 2 {
		t.Fatalf("refresh requests = %d, want 2 for unknown expiry", refreshRequests)
	}
	stored, err := unmarshalPersistedOIDCToken(store.items[auth.account], auth.identity)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IDToken != "saved-id" {
		t.Fatalf("persisted IDToken = %q, want preserved ID token", stored.IDToken)
	}
}

func TestOIDCRefreshRejectsMissingConfiguredTokenBeforePersisting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/token":
			writeJSON(t, w, map[string]interface{}{"refresh_token": "rotated-refresh"})
		}
	}))
	defer server.Close()

	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth(server.URL), server.Client(), store)
	saveTestToken(t, store, auth, &oidcToken{RefreshToken: "saved-refresh", obtainedAt: time.Now()})
	original := slices.Clone(store.items[auth.account])

	if _, err := auth.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "configured token") {
		t.Fatalf("Token() error = %v, want missing configured token", err)
	}
	if !slices.Equal(store.items[auth.account], original) {
		t.Fatal("invalid refresh response overwrote the saved login")
	}
}

func TestOIDCTransientRefreshFailurePreservesSavedLogin(t *testing.T) {
	deviceRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
		case "/device":
			deviceRequests++
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth(server.URL), server.Client(), store)
	saveTestToken(t, store, auth, &oidcToken{
		RefreshToken: "keep-refresh",
		ExpiresIn:    60,
		obtainedAt:   time.Now().Add(-time.Hour),
	})
	original := slices.Clone(store.items[auth.account])

	if _, err := auth.Token(context.Background()); err == nil {
		t.Fatal("expected transient refresh failure")
	}
	if deviceRequests != 0 {
		t.Fatalf("transient refresh error started %d device authorizations", deviceRequests)
	}
	if !slices.Equal(store.items[auth.account], original) {
		t.Fatal("transient refresh failure changed the saved login")
	}
}

func TestOIDCRefreshErrorRetryability(t *testing.T) {
	for _, code := range []string{"invalid_grant", "invalid_token", "unauthorized_client", "invalid_client"} {
		err := &oidcTokenEndpointError{Status: http.StatusBadRequest, Code: code}
		if !isNonRetryableOIDCRefreshError(err) {
			t.Errorf("HTTP 400 %s should be non-retryable", code)
		}
	}
	if isNonRetryableOIDCRefreshError(&oidcTokenEndpointError{Status: http.StatusServiceUnavailable, Code: "temporarily_unavailable"}) {
		t.Error("HTTP 503 should remain retryable")
	}
}

func TestOIDCInvalidRefreshTokenDeletesSavedLoginAndReauthenticates(t *testing.T) {
	t.Setenv("FOGHORN_OIDC_SKIP_BROWSER", "1")
	refreshRequests := 0
	deviceRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(t, w, map[string]string{
				"device_authorization_endpoint": "http://" + r.Host + "/device",
				"token_endpoint":                "http://" + r.Host + "/token",
			})
		case "/device":
			deviceRequests++
			writeJSON(t, w, map[string]interface{}{
				"device_code":      "new-device-code",
				"verification_uri": "https://login.example.test/device",
				"expires_in":       600,
			})
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") == "refresh_token" {
				refreshRequests++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			writeJSON(t, w, map[string]interface{}{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth(server.URL), server.Client(), store)
	saveTestToken(t, store, auth, &oidcToken{
		RefreshToken: "invalid-refresh",
		ExpiresIn:    60,
		obtainedAt:   time.Now().Add(-time.Hour),
	})

	// A rejected refresh discards the dead login and reports it; the sign-in it
	// makes necessary starts on the next attempt rather than inside this one.
	if _, err := auth.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "no longer valid") {
		t.Fatalf("Token() error = %v, want the saved login reported as invalid", err)
	}
	token := tokenWhenSignedIn(t, auth)
	if refreshRequests != 1 || deviceRequests != 1 {
		t.Fatalf("requests refresh=%d device=%d, want 1 each", refreshRequests, deviceRequests)
	}
	// Both the current slot and the legacy one are cleared, so logging out
	// cannot leave an item behind that a later start would migrate back in.
	if store.deleteCalls != 2 {
		t.Fatalf("Delete calls = %d, want 2 (current + legacy slot)", store.deleteCalls)
	}
	if token.AccessToken != "new-access" {
		t.Fatalf("AccessToken = %q, want reauthenticated token", token.AccessToken)
	}
	stored, err := unmarshalPersistedOIDCToken(store.items[auth.account], auth.identity)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshToken != "new-refresh" {
		t.Fatalf("saved RefreshToken = %q", stored.RefreshToken)
	}
}

func TestOIDCRetriesSaveAfterKeyringRecovery(t *testing.T) {
	store := newMemoryTokenStore()
	store.setFailures = 1
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)
	auth.token = &oidcToken{AccessToken: "access", ExpiresIn: 3600, obtainedAt: time.Now()}
	auth.tokenDirty = true
	auth.loadComplete = true

	if _, err := auth.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.items[auth.account]; ok {
		t.Fatal("first failed save unexpectedly wrote a credential")
	}
	if info := auth.SessionInfo(); info.StorageError == "" {
		t.Fatal("failed save was not reflected in session state")
	}
	if _, err := auth.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.items[auth.account]; !ok {
		t.Fatal("credential was not saved after keyring recovered")
	}
	if info := auth.SessionInfo(); info.StorageError != "" || !info.Saved {
		t.Fatalf("unexpected recovered session state: %#v", info)
	}
}

func TestOIDCKeyringReadFailureIsReportedAndRetried(t *testing.T) {
	store := newMemoryTokenStore()
	store.getErr = errors.New("test keyring locked")
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)

	if info := auth.SessionInfo(); info.StorageError == "" || info.Active {
		t.Fatalf("unexpected session state after read failure: %#v", info)
	}
	store.getErr = nil
	if info := auth.SessionInfo(); info.StorageError != "" {
		t.Fatalf("read was not retried after recovery: %#v", info)
	}
}

func TestOIDCPersistenceDisabledDoesNotReadOrWriteButForgetDeletes(t *testing.T) {
	store := newMemoryTokenStore()
	authConfig := persistentTestAuth("https://login.example.test")
	authConfig.PersistTokens = testBoolPointer(false)
	auth := newOIDCDeviceAuthenticatorWithStore("production", authConfig, http.DefaultClient, store)
	auth.token = &oidcToken{AccessToken: "access", ExpiresIn: 3600, obtainedAt: time.Now()}
	auth.tokenDirty = true
	store.items[auth.account] = []byte("old-secret")

	if _, err := auth.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.setCalls != 0 {
		t.Fatalf("Set calls = %d, want 0", store.setCalls)
	}
	if err := auth.Forget(); err != nil {
		t.Fatal(err)
	}
	// Both the current slot and the legacy one are cleared, so logging out
	// cannot leave an item behind that a later start would migrate back in.
	if store.deleteCalls != 2 {
		t.Fatalf("Delete calls = %d, want 2 (current + legacy slot)", store.deleteCalls)
	}
}

func TestOIDCForgetClearsMemoryAndSavedToken(t *testing.T) {
	store := newMemoryTokenStore()
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)
	auth.token = &oidcToken{AccessToken: "access", obtainedAt: time.Now()}
	auth.loadComplete = true
	auth.persisted = true
	store.items[auth.account] = []byte("secret")

	if err := auth.Forget(); err != nil {
		t.Fatal(err)
	}
	if auth.token != nil {
		t.Fatal("Forget() retained the in-memory token")
	}
	if _, ok := store.items[auth.account]; ok {
		t.Fatal("Forget() retained the saved token")
	}
	// Both the current slot and the legacy one are cleared, so logging out
	// cannot leave an item behind that a later start would migrate back in.
	if store.deleteCalls != 2 {
		t.Fatalf("Delete calls = %d, want 2 (current + legacy slot)", store.deleteCalls)
	}
}

func TestOIDCForgetCanRetryAfterKeyringRecovery(t *testing.T) {
	store := newMemoryTokenStore()
	store.deleteErr = errors.New("test keyring locked")
	auth := newOIDCDeviceAuthenticatorWithStore("production", persistentTestAuth("https://login.example.test"), http.DefaultClient, store)
	auth.token = &oidcToken{AccessToken: "access", obtainedAt: time.Now()}
	auth.loadComplete = true
	auth.persisted = true
	store.items[auth.account] = []byte("secret")

	if err := auth.Forget(); err == nil {
		t.Fatal("expected the first delete to fail")
	}
	if auth.token != nil {
		t.Fatal("failed Keychain deletion retained the in-memory token")
	}
	if info := auth.SessionInfo(); info.StorageError == "" {
		t.Fatal("delete failure was not exposed in session state")
	}

	store.deleteErr = nil
	if err := auth.Forget(); err != nil {
		t.Fatalf("retry Forget() error: %v", err)
	}
	if _, ok := store.items[auth.account]; ok {
		t.Fatal("retry retained the saved token")
	}
	if info := auth.SessionInfo(); info.StorageError != "" {
		t.Fatalf("storage error did not clear after recovery: %#v", info)
	}
}
