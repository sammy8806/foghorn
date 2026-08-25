package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"foghorn/internal/keyring"
	"foghorn/internal/provider"
)

func writeCLIConfig(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHandleCLIConfigCheck(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		path := writeCLIConfig(t, "sources: []\n")
		var stdout, stderr bytes.Buffer
		handled, code := handleCLI([]string{"config", "check"}, &stdout, &stderr)
		if !handled || code != 0 {
			t.Fatalf("handleCLI() = (%v, %d), stderr=%q", handled, code, stderr.String())
		}
		if got, want := stdout.String(), path+": OK\n"; got != want {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	})

	t.Run("diagnostics json", func(t *testing.T) {
		path := writeCLIConfig(t, "sources:\n  - type: alertmanager\n    url: https://alerts.example.test\n")
		var stdout, stderr bytes.Buffer
		_, code := handleCLI([]string{"config", "check", "--json"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("code = %d, want 1; stderr=%q", code, stderr.String())
		}
		var payload ConfigDiagnostics
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
		}
		if payload.Path != path || payload.Fingerprint == "" || len(payload.Items) != 1 || !payload.Items[0].Dropped {
			t.Fatalf("payload = %#v", payload)
		}
	})

	t.Run("substitution is strict", func(t *testing.T) {
		writeCLIConfig(t, "ui:\n  scale:\n    factor: 9\n")
		var stdout, stderr bytes.Buffer
		_, code := handleCLI([]string{"config", "check"}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stdout.String(), "ui.scale.factor") || !strings.Contains(stdout.String(), "using default") {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("unparseable", func(t *testing.T) {
		writeCLIConfig(t, "sources: [oh no\n")
		var stdout, stderr bytes.Buffer
		_, code := handleCLI([]string{"config", "check"}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stdout.String(), "config unusable") || !strings.Contains(stdout.String(), "parsing config") {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("missing", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		var stdout, stderr bytes.Buffer
		_, code := handleCLI([]string{"config", "check"}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stdout.String(), "config unusable") || !strings.Contains(stdout.String(), "reading config") {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
}

func TestAuthCLIWarnsAndContinuesForConfigDiagnostics(t *testing.T) {
	writeCLIConfig(t, "resolvers:\n  - name: stale\n    field: label:cluster\n    command: ./resolve\n    args: ['{{.Value}}']\n    stdin: value\n")
	var stdout, stderr bytes.Buffer
	_, code := handleCLI([]string{"auth", "list"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stderr.String(), "config warning") || !strings.Contains(stdout.String(), "No sources") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

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

func (f *fakeCLIKeyring) MaxSecretSize() int { return 0 }

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
	cfg, err := loadCLIConfig(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	account := provider.OIDCTokenAccount(cfg.Sources[0].Name)
	identity := provider.OIDCTokenIdentity(cfg.Sources[0].Name, cfg.Sources[0].Auth)
	storedToken, err := json.Marshal(map[string]interface{}{
		"version":       2,
		"identity":      identity,
		"refresh_token": "secret",
		"obtained_at":   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	store.items[account] = storedToken
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

	store.items[account], err = json.Marshal(map[string]interface{}{
		"version":       2,
		"identity":      "identity-from-an-old-configuration",
		"refresh_token": "secret",
		"obtained_at":   time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := loginStatus(cfg.Sources[0], store); status != "not saved" {
		t.Fatalf("identity-mismatched login status = %q, want not saved", status)
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
