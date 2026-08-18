// Package keyring stores application secrets in the operating system's secure
// credential store. The platform implementation deliberately exposes only the
// small generic-password surface Foghorn needs for OIDC token persistence.
package keyring

import "errors"

const OIDCService = "de.sammy8806.foghorn.oidc"

var (
	ErrNotFound    = errors.New("keyring item not found")
	ErrUnsupported = errors.New("keyring is not supported on this platform")
)

// Store is the minimal secret-store contract used by the OIDC authenticator.
// Implementations must never include secret values in returned errors.
type Store interface {
	Get(account string) ([]byte, error)
	Set(account string, secret []byte) error
	Delete(account string) error
}

// NewOIDCStore returns the platform store used for persisted OIDC tokens.
func NewOIDCStore() Store {
	return newStore(OIDCService)
}

// Supported reports whether this build has a native secure-store
// implementation. OIDC persistence defaults on only when this is true.
func Supported() bool { return supported() }
