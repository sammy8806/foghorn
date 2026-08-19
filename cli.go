package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"foghorn/internal/config"
	"foghorn/internal/keyring"
	"foghorn/internal/provider"
)

const cliUsage = `Usage:
  foghorn [--version|-v]
  foghorn auth list
  foghorn auth clear <source>
  foghorn auth clear --all

Commands manage saved cookie and OIDC keyring logins.
Configured passwords and API tokens are not modified.`

var (
	newCLIKeyringStore  = keyring.NewOIDCStore
	cliKeyringSupported = keyring.Supported
)

// handleCLI handles commands that do not start the desktop application.
func handleCLI(args []string, stdout, stderr io.Writer) (handled bool, exitCode int) {
	if len(args) == 0 {
		return false, 0
	}

	switch args[0] {
	case "--version", "-version", "-v", "version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "version does not accept arguments")
			return true, 2
		}
		fmt.Fprintf(stdout, "foghorn %s\n", version)
		return true, 0
	case "--help", "-h", "help":
		fmt.Fprintln(stdout, cliUsage)
		return true, 0
	case "auth":
		return handleAuthCLI(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s\n", args[0], cliUsage)
		return true, 2
	}
}

func handleAuthCLI(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(stdout, cliUsage)
		return true, 0
	}
	if args[0] != "list" && args[0] != "clear" {
		fmt.Fprintf(stderr, "unknown auth command %q\n\n%s\n", args[0], cliUsage)
		return true, 2
	}

	cfg, err := loadCLIConfig()
	if err != nil {
		fmt.Fprintf(stderr, "foghorn: %v\n", err)
		return true, 1
	}
	sources := managedAuthSources(cfg)

	if args[0] == "list" {
		if len(args) != 1 {
			fmt.Fprintln(stderr, "auth list does not accept arguments")
			return true, 2
		}
		if len(sources) == 0 {
			fmt.Fprintln(stdout, "No sources with managed logins configured.")
			return true, 0
		}
		store := newCLIKeyringStore()
		for _, src := range sources {
			kind := normalizedAuthType(src)
			status, location := loginStatus(src, store)
			if kind == "oidc" && !cliKeyringSupported() {
				status = "unsupported"
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", src.Name, kind, status, location)
		}
		return true, 0
	}

	if len(args) != 2 || (args[1] == "" || strings.HasPrefix(args[1], "-") && args[1] != "--all") {
		fmt.Fprintln(stderr, "usage: foghorn auth clear <source|--all>")
		return true, 2
	}
	targets := sources
	if args[1] != "--all" {
		targets = nil
		for _, src := range sources {
			if src.Name == args[1] {
				targets = append(targets, src)
			}
		}
		if len(targets) == 0 {
			fmt.Fprintf(stderr, "foghorn: no source with a managed login named %q\n", args[1])
			return true, 1
		}
	}

	removed := 0
	seen := make(map[string]bool)
	store := newCLIKeyringStore()
	for _, src := range targets {
		identity := loginIdentity(src)
		if seen[identity] {
			continue
		}
		seen[identity] = true
		wasRemoved, err := clearSavedLogin(src, store)
		if err != nil {
			fmt.Fprintf(stderr, "foghorn: clearing login for %q: %v\n", src.Name, err)
			return true, 1
		}
		if wasRemoved {
			removed++
		}
	}
	fmt.Fprintf(stdout, "Cleared %d saved login(s).\n", removed)
	return true, 0
}

func loadCLIConfig() (*config.Config, error) {
	path := configPath()
	config.MigrateLegacyPath(path)
	cfg, err := config.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return config.Default(), nil
	}
	return cfg, err
}

func managedAuthSources(cfg *config.Config) []config.SourceConfig {
	sources := make([]config.SourceConfig, 0)
	for _, src := range cfg.Sources {
		switch normalizedAuthType(src) {
		case "cookie", "oidc":
			sources = append(sources, src)
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	return sources
}

func normalizedAuthType(src config.SourceConfig) string {
	return strings.ToLower(strings.TrimSpace(src.Auth.Type))
}

func loginIdentity(src config.SourceConfig) string {
	if normalizedAuthType(src) == "oidc" {
		return "oidc:" + provider.OIDCTokenAccount(src.Name, src.Auth)
	}
	return "cookie:" + provider.CookieFilePath(src)
}

func loginStatus(src config.SourceConfig, store keyring.Store) (status, location string) {
	if normalizedAuthType(src) == "oidc" {
		location = "system keyring"
		if !cliKeyringSupported() {
			return "unsupported", location
		}
		secret, err := store.Get(provider.OIDCTokenAccount(src.Name, src.Auth))
		for i := range secret {
			secret[i] = 0
		}
		if err == nil {
			return "saved", location
		}
		if errors.Is(err, keyring.ErrNotFound) {
			return "not saved", location
		}
		return "unavailable", location
	}

	path := provider.CookieFilePath(src)
	if _, err := os.Stat(path); err == nil {
		return "saved", path
	} else if errors.Is(err, os.ErrNotExist) {
		return "not saved", path
	}
	return "unavailable", path
}

func clearSavedLogin(src config.SourceConfig, store keyring.Store) (bool, error) {
	if normalizedAuthType(src) == "oidc" {
		if !cliKeyringSupported() {
			return false, keyring.ErrUnsupported
		}
		account := provider.OIDCTokenAccount(src.Name, src.Auth)
		secret, err := store.Get(account)
		for i := range secret {
			secret[i] = 0
		}
		if errors.Is(err, keyring.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if err := store.Delete(account); err != nil {
			return false, err
		}
		return true, nil
	}

	err := os.Remove(provider.CookieFilePath(src))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
