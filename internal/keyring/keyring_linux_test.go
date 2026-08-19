//go:build linux

package keyring

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	systemkeyring "github.com/zalando/go-keyring"
)

type fakeLinuxBackend struct {
	items     map[string]string
	getErr    error
	setErr    error
	deleteErr error
}

func newFakeLinuxBackend() *fakeLinuxBackend {
	return &fakeLinuxBackend{items: make(map[string]string)}
}

func (b *fakeLinuxBackend) key(service, account string) string {
	return service + "\x00" + account
}

func (b *fakeLinuxBackend) Get(service, account string) (string, error) {
	if b.getErr != nil {
		return "", b.getErr
	}
	secret, ok := b.items[b.key(service, account)]
	if !ok {
		return "", systemkeyring.ErrNotFound
	}
	return secret, nil
}

func (b *fakeLinuxBackend) Set(service, account, secret string) error {
	if b.setErr != nil {
		return b.setErr
	}
	b.items[b.key(service, account)] = secret
	return nil
}

func (b *fakeLinuxBackend) Delete(service, account string) error {
	if b.deleteErr != nil {
		return b.deleteErr
	}
	key := b.key(service, account)
	if _, ok := b.items[key]; !ok {
		return systemkeyring.ErrNotFound
	}
	delete(b.items, key)
	return nil
}

func TestLinuxStoreRoundTrip(t *testing.T) {
	backend := newFakeLinuxBackend()
	store := &linuxStore{service: "de.sammy8806.foghorn.test", backend: backend}
	account := "oidc-account"
	first := []byte("first token response")
	second := []byte("rotated token response")

	if _, err := store.Get(account); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if err := store.Set(account, first); err != nil {
		t.Fatalf("Set(first): %v", err)
	}
	if got, err := store.Get(account); err != nil || !slices.Equal(got, first) {
		t.Fatalf("Get(first) = %q, %v", got, err)
	}
	if err := store.Set(account, second); err != nil {
		t.Fatalf("Set(update): %v", err)
	}
	if got, err := store.Get(account); err != nil || !slices.Equal(got, second) {
		t.Fatalf("Get(update) = %q, %v", got, err)
	}
	if err := store.Delete(account); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.Delete(account); err != nil {
		t.Fatalf("Delete(missing) = %v, want nil", err)
	}
}

func TestLinuxBackendMetadata(t *testing.T) {
	if !Supported() {
		t.Fatal("Linux Secret Service backend must be supported")
	}
	if got := BackendName(); got != "Linux Secret Service" {
		t.Fatalf("BackendName() = %q, want %q", got, "Linux Secret Service")
	}
}

func TestLinuxStoreDoesNotExposeSecretInErrors(t *testing.T) {
	backend := newFakeLinuxBackend()
	backend.setErr = errors.New("session keyring unavailable")
	store := &linuxStore{service: "de.sammy8806.foghorn.test", backend: backend}
	secret := []byte("must-not-appear-in-errors")

	err := store.Set("oidc-account", secret)
	if err == nil {
		t.Fatal("expected Set() to fail")
	}
	if strings.Contains(err.Error(), string(secret)) {
		t.Fatalf("Set() error exposed secret material: %v", err)
	}
}

// This test touches the user's real Secret Service and is opt-in so ordinary
// unit tests work on headless Linux systems without a D-Bus session or keyring.
func TestLinuxSecretServiceIntegration(t *testing.T) {
	if os.Getenv("FOGHORN_TEST_LINUX_SECRET_SERVICE") != "1" {
		t.Skip("set FOGHORN_TEST_LINUX_SECRET_SERVICE=1 to test the real Secret Service")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(random)
	store := newStore("de.sammy8806.foghorn.test." + suffix)
	account := "round-trip-" + suffix
	t.Cleanup(func() {
		if err := store.Delete(account); err != nil {
			t.Errorf("cleanup Secret Service item: %v", err)
		}
	})

	first := []byte("random non-production OIDC token material")
	if err := store.Set(account, first); err != nil {
		t.Fatalf("Set(first): %v", err)
	}
	if got, err := store.Get(account); err != nil || !slices.Equal(got, first) {
		t.Fatalf("Get(first) = %q, %v", got, err)
	}
	second := []byte("rotated random non-production token material")
	if err := store.Set(account, second); err != nil {
		t.Fatalf("Set(update): %v", err)
	}
	if got, err := store.Get(account); err != nil || !slices.Equal(got, second) {
		t.Fatalf("Get(update) = %q, %v", got, err)
	}
	if err := store.Delete(account); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(account); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrNotFound", err)
	}
}
