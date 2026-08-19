//go:build linux

package keyring

import (
	"errors"
	"fmt"

	systemkeyring "github.com/zalando/go-keyring"
)

type linuxKeyringBackend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

type secretServiceBackend struct{}

func (secretServiceBackend) Get(service, account string) (string, error) {
	return systemkeyring.Get(service, account)
}

func (secretServiceBackend) Set(service, account, secret string) error {
	return systemkeyring.Set(service, account, secret)
}

func (secretServiceBackend) Delete(service, account string) error {
	return systemkeyring.Delete(service, account)
}

type linuxStore struct {
	service string
	backend linuxKeyringBackend
}

func newStore(service string) Store {
	return &linuxStore{service: service, backend: secretServiceBackend{}}
}

func supported() bool { return true }

func backendName() string { return "Linux Secret Service" }

func (s *linuxStore) Get(account string) ([]byte, error) {
	secret, err := s.backend.Get(s.service, account)
	if errors.Is(err, systemkeyring.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("Linux Secret Service read failed: %w", err)
	}
	return []byte(secret), nil
}

func (s *linuxStore) Set(account string, secret []byte) error {
	if err := s.backend.Set(s.service, account, string(secret)); err != nil {
		return fmt.Errorf("Linux Secret Service save failed: %w", err)
	}
	return nil
}

func (s *linuxStore) Delete(account string) error {
	err := s.backend.Delete(s.service, account)
	if errors.Is(err, systemkeyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Linux Secret Service delete failed: %w", err)
	}
	return nil
}
