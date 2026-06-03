package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"foghorn/internal/config"
)

type cookieLoginFunc func(ctx context.Context, redirectURL string, returnURL string) error

type cookieAuthenticator struct {
	cfg        config.SourceConfig
	jar        *persistentCookieJar
	login      cookieLoginFunc
	loginMutex sync.Mutex
}

func newCookieAuthenticator(cfg config.SourceConfig) (*cookieAuthenticator, error) {
	if strings.ToLower(strings.TrimSpace(cfg.Auth.Type)) != "cookie" {
		return nil, nil
	}
	jar, err := newPersistentCookieJar(cookieFilePath(cfg))
	if err != nil {
		return nil, err
	}
	return &cookieAuthenticator{
		cfg:   cfg,
		jar:   jar,
		login: defaultCookieLogin,
	}, nil
}

func (a *cookieAuthenticator) HandleRedirect(ctx context.Context, redirectURL string, returnURL string) error {
	if a == nil {
		return errors.New("cookie auth is not configured")
	}
	a.loginMutex.Lock()
	defer a.loginMutex.Unlock()
	if err := a.login(ctx, redirectURL, returnURL); err != nil {
		return err
	}
	return a.jar.Save()
}

func defaultCookieLogin(_ context.Context, redirectURL string, _ string) error {
	return fmt.Errorf("cookie auth requires an embedded login browser; login URL: %s", redirectURL)
}

func isCookieAuth(auth config.AuthConfig) bool {
	return strings.ToLower(strings.TrimSpace(auth.Type)) == "cookie"
}

func cookieFilePath(cfg config.SourceConfig) string {
	if configured := strings.TrimSpace(cfg.Auth.CookieFile); configured != "" {
		return configured
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, cfg.Name)
	if name == "" {
		name = "source"
	}
	return filepath.Join(dir, "foghorn", "cookies", name+".json")
}

type persistentCookieJar struct {
	inner *cookiejar.Jar
	path  string
	mu    sync.Mutex
	store map[string][]serialCookie
}

type serialCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"http_only,omitempty"`
}

func newPersistentCookieJar(path string) (*persistentCookieJar, error) {
	inner, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	jar := &persistentCookieJar{inner: inner, path: path, store: map[string][]serialCookie{}}
	if err := jar.Load(); err != nil {
		return nil, err
	}
	return jar, nil
}

func (j *persistentCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.inner.SetCookies(u, cookies)
	j.mu.Lock()
	defer j.mu.Unlock()
	key := cookieStoreKey(u)
	serial := make([]serialCookie, 0, len(cookies))
	for _, c := range cookies {
		serial = append(serial, serialCookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Secure:   c.Secure,
			HTTPOnly: c.HttpOnly,
		})
	}
	j.store[key] = mergeCookies(j.store[key], serial)
}

func (j *persistentCookieJar) Cookies(u *url.URL) []*http.Cookie {
	return j.inner.Cookies(u)
}

func (j *persistentCookieJar) Load() error {
	if j.path == "" {
		return nil
	}
	data, err := os.ReadFile(j.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("loading cookie auth jar: %w", err)
	}
	var store map[string][]serialCookie
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("parsing cookie auth jar: %w", err)
	}
	for key, cookies := range store {
		u, err := url.Parse(key)
		if err != nil {
			continue
		}
		j.inner.SetCookies(u, deserializeCookies(cookies))
	}
	j.store = store
	return nil
}

func (j *persistentCookieJar) Save() error {
	if j.path == "" {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(j.path), 0o700); err != nil {
		return fmt.Errorf("creating cookie auth dir: %w", err)
	}
	data, err := json.MarshalIndent(j.store, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing cookie auth jar: %w", err)
	}
	return os.WriteFile(j.path, data, 0o600)
}

func deserializeCookies(in []serialCookie) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(in))
	for _, c := range in {
		cookies = append(cookies, &http.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Domain:   c.Domain,
			Secure:   c.Secure,
			HttpOnly: c.HTTPOnly,
		})
	}
	return cookies
}

func mergeCookies(existing []serialCookie, incoming []serialCookie) []serialCookie {
	merged := make(map[string]serialCookie, len(existing)+len(incoming))
	for _, c := range existing {
		merged[c.Name+"\x00"+c.Path+"\x00"+c.Domain] = c
	}
	for _, c := range incoming {
		merged[c.Name+"\x00"+c.Path+"\x00"+c.Domain] = c
	}
	out := make([]serialCookie, 0, len(merged))
	for _, c := range merged {
		out = append(out, c)
	}
	return out
}

func cookieStoreKey(u *url.URL) string {
	return u.Scheme + "://" + u.Host
}
