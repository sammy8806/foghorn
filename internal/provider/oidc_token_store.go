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

const persistedOIDCTokenVersion = 1

type persistedOIDCToken struct {
	Version      int       `json:"version"`
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

func oidcTokenAccount(source string, auth config.AuthConfig) string {
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
	return "v1:" + hex.EncodeToString(digest[:])
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

func marshalPersistedOIDCToken(token *oidcToken) ([]byte, error) {
	if token == nil {
		return nil, errors.New("cannot persist a nil OIDC token")
	}
	return json.Marshal(persistedOIDCToken{
		Version:      persistedOIDCTokenVersion,
		AccessToken:  token.AccessToken,
		IDToken:      token.IDToken,
		TokenType:    token.TokenType,
		ExpiresIn:    token.ExpiresIn,
		RefreshToken: token.RefreshToken,
		ObtainedAt:   token.obtainedAt,
	})
}

func unmarshalPersistedOIDCToken(encoded []byte) (*oidcToken, error) {
	var stored persistedOIDCToken
	if err := json.Unmarshal(encoded, &stored); err != nil {
		return nil, fmt.Errorf("decoding saved OIDC token: %w", err)
	}
	if stored.Version != persistedOIDCTokenVersion {
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
		a.loadComplete = true
		a.persisted = false
		a.recordStorageSuccessLocked()
		return
	}
	if err != nil {
		a.recordStorageErrorLocked("read", err)
		return
	}
	token, err := unmarshalPersistedOIDCToken(encoded)
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

func (a *oidcDeviceAuthenticator) savePersistedTokenLocked() {
	if !a.persistenceEnabled || !a.tokenDirty || a.token == nil {
		return
	}
	encoded, err := marshalPersistedOIDCToken(a.token)
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
}

func (a *oidcDeviceAuthenticator) deletePersistedTokenLocked() error {
	// Deletion intentionally ignores persist_tokens. A user who disables
	// persistence must still be able to remove an item saved by an older config.
	if !a.storeSupported {
		return nil
	}
	if err := a.store.Delete(a.account); err != nil {
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
