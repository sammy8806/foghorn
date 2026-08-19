//go:build windows

package keyring

import (
	"errors"
	"fmt"

	native "github.com/zalando/go-keyring"
)

// windowsStore stores credentials in Windows Credential Manager.
type windowsStore struct {
	service string
}

func newStore(service string) Store { return windowsStore{service: service} }
func supported() bool               { return true }
func backendName() string           { return "Windows Credential Manager" }

func (s windowsStore) Get(account string) ([]byte, error) {
	secret, err := native.Get(s.service, account)
	if errors.Is(err, native.ErrNotFound) {
		return nil, ErrNotFound
	}
	if errors.Is(err, native.ErrUnsupportedPlatform) {
		return nil, ErrUnsupported
	}
	if err != nil {
		return nil, fmt.Errorf("Windows Credential Manager read failed: %w", err)
	}
	if secret == "" {
		return nil, ErrNotFound
	}
	return []byte(secret), nil
}

func (s windowsStore) Set(account string, secret []byte) error {
	err := native.Set(s.service, account, string(secret))
	if errors.Is(err, native.ErrUnsupportedPlatform) {
		return ErrUnsupported
	}
	if err != nil {
		return fmt.Errorf("Windows Credential Manager save failed: %w", err)
	}
	return nil
}

func (s windowsStore) Delete(account string) error {
	err := native.Delete(s.service, account)
	if errors.Is(err, native.ErrNotFound) {
		return nil
	}
	if errors.Is(err, native.ErrUnsupportedPlatform) {
		return ErrUnsupported
	}
	if err != nil {
		return fmt.Errorf("Windows Credential Manager delete failed: %w", err)
	}
	return nil
}
