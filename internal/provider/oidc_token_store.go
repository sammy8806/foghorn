package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/keyring"
)

// persistedOIDCTokenVersion 2 keys the credential-store item by source name and
// records the login identity inside the payload. Version 1 keyed the item by the
// whole identity, which orphaned an item on every issuer/client/scope edit.
const persistedOIDCTokenVersion = 2

// errOIDCIdentityMismatch reports a saved login minted for a different login
// configuration than the one currently configured for the source.
var errOIDCIdentityMismatch = errors.New("saved OIDC login belongs to a different login configuration")

type persistedOIDCToken struct {
	Version      int       `json:"version"`
	Identity     string    `json:"identity,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ObtainedAt   time.Time `json:"obtained_at"`
}

type oidcTokenIdentity struct {
	Source                 string   `json:"source"`
	IssuerURL              string   `json:"issuer_url,omitempty"`
	DeviceAuthorizationURL string   `json:"device_authorization_url,omitempty"`
	TokenURL               string   `json:"token_url,omitempty"`
	ClientID               string   `json:"client_id"`
	Scopes                 []string `json:"scopes,omitempty"`
	UseIDToken             bool     `json:"use_id_token,omitempty"`
}

func oidcPersistenceEnabled(auth config.AuthConfig) bool {
	if auth.PersistTokens != nil {
		return *auth.PersistTokens && keyring.Supported()
	}
	return keyring.Supported()
}

// OIDCTokenAccount returns the keyring account for an OIDC source: one slot per
// source name, so editing the issuer, client or scopes reuses the slot instead
// of stranding an item the user can neither see nor delete. Which login the slot
// holds is recorded in the payload as OIDCTokenIdentity and checked on load.
// The account contains no secret material.
func OIDCTokenAccount(source string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(source)))
	return "v2:" + hex.EncodeToString(digest[:])
}

// OIDCLegacyTokenAccount returns the pre-v2 account for a source, where the
// whole login identity was hashed into the account name. Retained so an existing
// saved login can be migrated into the v2 slot and then removed.
func OIDCLegacyTokenAccount(source string, auth config.AuthConfig) string {
	return "v1:" + OIDCTokenIdentity(source, auth)
}

// OIDCTokenIdentity fingerprints the login configuration a token was issued
// for: source, endpoints, client, scopes and which token is sent. It is stored
// beside the token so a restored login can be checked against the current
// config, and contains no secret material.
func OIDCTokenIdentity(source string, auth config.AuthConfig) string {
	scopes := make([]string, 0, len(auth.Scopes))
	seen := make(map[string]struct{}, len(auth.Scopes))
	for _, scope := range auth.Scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	identity := oidcTokenIdentity{
		Source:                 strings.TrimSpace(source),
		IssuerURL:              normalizeOIDCIdentityURL(auth.IssuerURL),
		DeviceAuthorizationURL: normalizeOIDCIdentityURL(auth.DeviceAuthorizationURL),
		TokenURL:               normalizeOIDCIdentityURL(auth.TokenURL),
		ClientID:               strings.TrimSpace(auth.ClientID),
		Scopes:                 scopes,
		UseIDToken:             auth.UseIDToken,
	}
	encoded, _ := json.Marshal(identity)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func normalizeOIDCIdentityURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return trimmed
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

// oidcTokenTooLargeError reports a payload the credential store refuses even
// after dropping every token that is not required to restore the login.
type oidcTokenTooLargeError struct {
	size  int
	limit int
}

func (e *oidcTokenTooLargeError) Error() string {
	return fmt.Sprintf("saved login needs %d bytes but this credential store accepts at most %d: the identity provider issues tokens too large to persist", e.size, e.limit)
}

// marshalPersistedOIDCTokenWithin encodes the smallest payload that fits limit
// (0 means unlimited). Only the refresh token has to survive a restart; the
// access and ID tokens merely save one refresh round trip at startup, so they
// are dropped in turn rather than failing the save outright. This keeps token
// persistence working on Windows, whose Credential Manager caps a credential
// blob at 2560 bytes while a single JWT with group claims can exceed that.
func marshalPersistedOIDCTokenWithin(token *oidcToken, identity string, useIDToken bool, limit int) ([]byte, error) {
	if token == nil {
		return nil, errors.New("cannot persist a nil OIDC token")
	}
	tiers := []struct{ keepAccess, keepID bool }{{true, true}}
	if useIDToken {
		tiers = append(tiers, struct{ keepAccess, keepID bool }{keepAccess: false, keepID: true})
	} else {
		tiers = append(tiers, struct{ keepAccess, keepID bool }{keepAccess: true, keepID: false})
	}
	tiers = append(tiers, struct{ keepAccess, keepID bool }{keepAccess: false, keepID: false})

	var largest int
	for _, tier := range tiers {
		trimmed := *token
		if !tier.keepAccess {
			trimmed.AccessToken = ""
		}
		if !tier.keepID {
			trimmed.IDToken = ""
		}
		if trimmed.AccessToken == "" && trimmed.IDToken == "" && trimmed.RefreshToken == "" {
			continue
		}
		// Without the token this source actually sends, a restored login must go
		// straight to a refresh instead of being handed out as though it were live.
		if (useIDToken && !tier.keepID) || (!useIDToken && !tier.keepAccess) {
			trimmed.ExpiresIn = 0
		}
		encoded, err := marshalPersistedOIDCToken(&trimmed, identity)
		if err != nil {
			return nil, err
		}
		if limit <= 0 || len(encoded) <= limit {
			return encoded, nil
		}
		largest = len(encoded)
	}
	return nil, &oidcTokenTooLargeError{size: largest, limit: limit}
}

func marshalPersistedOIDCToken(token *oidcToken, identity string) ([]byte, error) {
	if token == nil {
		return nil, errors.New("cannot persist a nil OIDC token")
	}
	return json.Marshal(persistedOIDCToken{
		Version:      persistedOIDCTokenVersion,
		Identity:     identity,
		AccessToken:  token.AccessToken,
		IDToken:      token.IDToken,
		TokenType:    token.TokenType,
		ExpiresIn:    token.ExpiresIn,
		RefreshToken: token.RefreshToken,
		ObtainedAt:   token.obtainedAt,
	})
}

// unmarshalPersistedOIDCToken decodes a saved payload. wantIdentity, when set,
// must match the fingerprint recorded beside the token; the check fails closed
// so a token minted for one login configuration is never presented for another.
// Version 1 payloads carry no fingerprint because their account name was derived
// from the identity, which already proved the match.
func unmarshalPersistedOIDCToken(encoded []byte, wantIdentity string) (*oidcToken, error) {
	var stored persistedOIDCToken
	if err := json.Unmarshal(encoded, &stored); err != nil {
		return nil, fmt.Errorf("decoding saved OIDC token: %w", err)
	}
	switch stored.Version {
	case 1:
	case persistedOIDCTokenVersion:
		if wantIdentity != "" && stored.Identity != wantIdentity {
			return nil, errOIDCIdentityMismatch
		}
	default:
		return nil, fmt.Errorf("unsupported saved OIDC token version %d", stored.Version)
	}
	if stored.ObtainedAt.IsZero() {
		return nil, errors.New("saved OIDC token is missing its acquisition time")
	}
	if strings.TrimSpace(stored.AccessToken) == "" &&
		strings.TrimSpace(stored.IDToken) == "" &&
		strings.TrimSpace(stored.RefreshToken) == "" {
		return nil, errors.New("saved OIDC token contains no tokens")
	}
	return &oidcToken{
		AccessToken:  stored.AccessToken,
		IDToken:      stored.IDToken,
		TokenType:    stored.TokenType,
		ExpiresIn:    stored.ExpiresIn,
		RefreshToken: stored.RefreshToken,
		obtainedAt:   stored.ObtainedAt,
	}, nil
}

func (a *oidcDeviceAuthenticator) loadPersistedTokenLocked() {
	if !a.persistenceEnabled || a.loadComplete || a.token != nil {
		return
	}
	encoded, err := a.store.Get(a.account)
	if errors.Is(err, keyring.ErrNotFound) {
		if a.loadLegacyTokenLocked() {
			return
		}
		a.loadComplete = true
		a.persisted = false
		a.recordStorageSuccessLocked()
		return
	}
	if err != nil {
		a.recordStorageErrorLocked("read", err)
		return
	}
	token, err := unmarshalPersistedOIDCToken(encoded, a.identity)
	if errors.Is(err, errOIDCIdentityMismatch) {
		// The source's login configuration changed (issuer, client_id, scopes,
		// use_id_token). Fail closed rather than presenting a token minted for a
		// different identity. Because the slot is keyed by source name, the next
		// successful login overwrites it instead of orphaning an item the user
		// can neither list nor clear.
		log.Printf("oidc: saved login for source %q was issued for a different login configuration; signing in again", a.source)
		a.loadComplete = true
		a.persisted = false
		a.recordStorageSuccessLocked()
		return
	}
	if err != nil {
		a.loadComplete = true
		a.persisted = true
		a.recordStorageErrorLocked("decode", err)
		return
	}
	a.token = token
	a.loadComplete = true
	a.persisted = true
	a.recordStorageSuccessLocked()
	log.Printf("oidc: restored saved login for source %q from %s", a.source, a.storageBackend)
}

// loadLegacyTokenLocked migrates a pre-v2 item, whose account name encoded the
// whole login identity, into the slot keyed by source name. The legacy item is
// removed only after the migrated copy has been saved, so a failed save cannot
// lose the login.
func (a *oidcDeviceAuthenticator) loadLegacyTokenLocked() bool {
	if a.legacyAccount == "" {
		return false
	}
	encoded, err := a.store.Get(a.legacyAccount)
	if err != nil {
		return false
	}
	// The legacy account name is itself derived from the identity, so reaching a
	// stored item under it already proves the configuration matches.
	token, err := unmarshalPersistedOIDCToken(encoded, "")
	if err != nil {
		return false
	}
	a.token = token
	a.tokenDirty = true
	a.loadComplete = true
	a.persisted = true
	a.legacyPending = true
	a.recordStorageSuccessLocked()
	log.Printf("oidc: migrating saved login for source %q to the current credential-store layout", a.source)
	return true
}

func (a *oidcDeviceAuthenticator) savePersistedTokenLocked() {
	if !a.persistenceEnabled || !a.tokenDirty || a.token == nil {
		return
	}
	encoded, err := marshalPersistedOIDCTokenWithin(a.token, a.identity, a.cfg.UseIDToken, a.store.MaxSecretSize())
	if err == nil {
		err = a.store.Set(a.account, encoded)
	}
	if err != nil {
		a.recordStorageErrorLocked("save", err)
		return
	}
	a.tokenDirty = false
	a.loadComplete = true
	a.persisted = true
	a.recordStorageSuccessLocked()
	a.deleteLegacyTokenLocked()
}

// deleteLegacyTokenLocked removes a migrated pre-v2 item once its replacement
// has been written.
func (a *oidcDeviceAuthenticator) deleteLegacyTokenLocked() {
	if !a.legacyPending {
		return
	}
	a.legacyPending = false
	if err := a.store.Delete(a.legacyAccount); err != nil {
		log.Printf("oidc: source %q could not remove the migrated legacy credential-store item: %v", a.source, err)
	}
}

func (a *oidcDeviceAuthenticator) deletePersistedTokenLocked() error {
	// Deletion intentionally ignores persist_tokens. A user who disables
	// persistence must still be able to remove an item saved by an older config.
	if !a.storeSupported {
		return nil
	}
	a.legacyPending = false
	// Clear both layouts: logging out must not leave a pre-v2 item behind that
	// would be migrated back in on the next start.
	err := a.store.Delete(a.account)
	if legacyErr := a.store.Delete(a.legacyAccount); err == nil {
		err = legacyErr
	}
	if err != nil {
		a.recordStorageErrorLocked("delete", err)
		return err
	}
	a.persisted = false
	a.loadComplete = true
	a.recordStorageSuccessLocked()
	return nil
}

func (a *oidcDeviceAuthenticator) recordStorageErrorLocked(operation string, err error) {
	message := fmt.Sprintf("%s: %v", operation, err)
	if message == a.storageError {
		return
	}
	a.storageError = message
	log.Printf("oidc: source %q %s %s failed; continuing with memory-only credentials: %v", a.source, a.storageBackend, operation, err)
}

func (a *oidcDeviceAuthenticator) recordStorageSuccessLocked() {
	if a.storageError != "" {
		log.Printf("oidc: source %q %s access recovered", a.source, a.storageBackend)
	}
	a.storageError = ""
}
