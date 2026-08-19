//go:build darwin && cgo

package keyring

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"testing"
)

// This test touches the user's login Keychain and is opt-in so ordinary unit
// tests never create credentials or trigger a macOS access prompt.
func TestMacOSKeyringRoundTrip(t *testing.T) {
	if os.Getenv("FOGHORN_TEST_MACOS_KEYCHAIN") != "1" {
		t.Skip("set FOGHORN_TEST_MACOS_KEYCHAIN=1 to test the real macOS Keychain")
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
			t.Errorf("cleanup Keychain item: %v", err)
		}
	})

	if _, err := store.Get(account); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrNotFound", err)
	}
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
