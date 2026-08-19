package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"foghorn/internal/keyring"
	"foghorn/internal/provider"
)

type fakeCLIKeyring struct {
	items map[string][]byte
}

func (f *fakeCLIKeyring) Get(account string) ([]byte, error) {
	secret, ok := f.items[account]
	if !ok {
		return nil, keyring.ErrNotFound
	}
	return append([]byte(nil), secret...), nil
}

func (f *fakeCLIKeyring) Set(account string, secret []byte) error {
	f.items[account] = append([]byte(nil), secret...)
	return nil
}

func (f *fakeCLIKeyring) Delete(account string) error {
	delete(f.items, account)
	return nil
}

func TestHandleCLIVersion(t *testing.T) {
	oldVersion := version
	version = "1.2.3-test"
	defer func() { version = oldVersion }()

	for _, flag := range []string{"--version", "-v"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			handled, code := handleCLI([]string{flag}, &stdout, &stderr)
			if !handled || code != 0 {
				t.Fatalf("handleCLI() = (%v, %d), want (true, 0); stderr=%q", handled, code, stderr.String())
			}
			if got, want := stdout.String(), "foghorn 1.2.3-test\n"; got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
		})
	}
}

func TestHandleCLIAuthListAndClear(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	cookiePath := filepath.Join(home, "saved-login.json")
	cfgPath := configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	configBody := "sources:\n  - name: production\n    type: alertmanager\n    url: https://alerts.example.test\n    auth:\n      type: cookie\n      cookie_file: " + cookiePath + "\n"
	if err := os.WriteFile(cfgPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookiePath, []byte(`{"https://alerts.example.test":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	handled, code := handleCLI([]string{"auth", "list"}, &stdout, &stderr)
	if !handled || code != 0 {
		t.Fatalf("auth list = (%v, %d), stderr=%q", handled, code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Managed logins (1)") ||
		!strings.Contains(got, "SOURCE") ||
		!strings.Contains(got, "production") ||
		!strings.Contains(got, "COOKIE") ||
		!strings.Contains(got, "Saved") ||
		!strings.Contains(got, cookiePath) {
		t.Fatalf("auth list output = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	_, code = handleCLI([]string{"auth", "clear", "production"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth clear code = %d, stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(cookiePath); !os.IsNotExist(err) {
		t.Fatalf("cookie file still exists or stat failed unexpectedly: %v", err)
	}
	if got, want := stdout.String(), "Cleared saved login for \"production\". Foghorn will ask you to sign in again.\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestHandleCLIAuthListAndClearOIDCKeyring(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	cfgPath := configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	configBody := "sources:\n  - name: sso\n    type: alertmanager\n    url: https://alerts.example.test\n    auth:\n      type: oidc\n      flow: device\n      issuer_url: https://login.example.test\n      client_id: foghorn\n"
	if err := os.WriteFile(cfgPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	store := &fakeCLIKeyring{items: map[string][]byte{}}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	account := provider.OIDCTokenAccount(cfg.Sources[0].Name, cfg.Sources[0].Auth)
	store.items[account] = []byte(`{"refresh_token":"secret"}`)
	oldStore, oldSupported := newCLIKeyringStore, cliKeyringSupported
	newCLIKeyringStore = func() keyring.Store { return store }
	cliKeyringSupported = func() bool { return true }
	t.Cleanup(func() {
		newCLIKeyringStore = oldStore
		cliKeyringSupported = oldSupported
	})

	var stdout, stderr bytes.Buffer
	_, code := handleCLI([]string{"auth", "list"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "sso") ||
		!strings.Contains(stdout.String(), "OIDC") ||
		!strings.Contains(stdout.String(), "Saved") ||
		!strings.Contains(stdout.String(), platformKeyringName()) {
		t.Fatalf("auth list code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	_, code = handleCLI([]string{"auth", "list", "--json"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), `"source": "sso"`) ||
		!strings.Contains(stdout.String(), `"storage": "`+platformKeyringName()+`"`) {
		t.Fatalf("auth list --json code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	_, code = handleCLI([]string{"auth", "clear", "sso"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth clear code=%d stderr=%q", code, stderr.String())
	}
	if _, ok := store.items[account]; ok {
		t.Fatal("OIDC keyring item was not deleted")
	}
}

func TestHandleCLIAuthClearAllSkipsUnsupportedOIDC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	cookiePath := filepath.Join(home, "saved-login.json")
	cfgPath := configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatal(err)
	}
	configBody := "sources:\n" +
		"  - name: a-sso\n" +
		"    type: alertmanager\n" +
		"    url: https://alerts.example.test\n" +
		"    auth:\n" +
		"      type: oidc\n" +
		"      flow: device\n" +
		"      issuer_url: https://login.example.test\n" +
		"      client_id: foghorn\n" +
		"  - name: z-cookie\n" +
		"    type: alertmanager\n" +
		"    url: https://alerts.example.test\n" +
		"    auth:\n" +
		"      type: cookie\n" +
		"      cookie_file: " + cookiePath + "\n"
	if err := os.WriteFile(cfgPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookiePath, []byte(`{"https://alerts.example.test":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	oldSupported := cliKeyringSupported
	cliKeyringSupported = func() bool { return false }
	t.Cleanup(func() { cliKeyringSupported = oldSupported })

	var stdout, stderr bytes.Buffer
	_, code := handleCLI([]string{"auth", "clear", "--all"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth clear --all code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(cookiePath); !os.IsNotExist(err) {
		t.Fatalf("cookie file still exists or stat failed unexpectedly: %v", err)
	}
	if got, want := stdout.String(), "Cleared 1 saved login(s); skipped 1 unsupported login(s).\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	_, code = handleCLI([]string{"auth", "clear", "a-sso"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), keyring.ErrUnsupported.Error()) {
		t.Fatalf("individual unsupported clear code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
