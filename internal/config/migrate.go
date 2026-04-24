package config

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// MigrateLegacyPath moves the config from the legacy ~/.config/foghorn/config.yaml
// location to newPath when the two differ (i.e. on macOS and Windows). It is a
// no-op on Linux where the paths are identical.
func MigrateLegacyPath(newPath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldPath := filepath.Join(home, ".config", "foghorn", "config.yaml")
	Migrate(oldPath, newPath)
}

// Migrate moves oldPath to newPath. It is a no-op when:
//   - the paths are the same (Linux: old == new),
//   - newPath already exists (user is already set up correctly), or
//   - oldPath does not exist (fresh install with no legacy config).
//
// It tries os.Rename first and falls back to a copy+delete for cross-device
// moves (e.g. %APPDATA% on a different drive on Windows).
func Migrate(oldPath, newPath string) {
	if oldPath == newPath {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		return // new location already populated, nothing to do
	}
	if _, err := os.Stat(oldPath); err != nil {
		return // no legacy config to migrate
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err != nil {
		log.Printf("config: migration: could not create %s: %v", filepath.Dir(newPath), err)
		return
	}

	if err := os.Rename(oldPath, newPath); err == nil {
		log.Printf("config: migrated config from %s to %s", oldPath, newPath)
		return
	}

	// Rename failed (likely a cross-device move): fall back to copy + delete.
	if err := copyFile(oldPath, newPath); err != nil {
		log.Printf("config: migration: could not copy config from %s to %s: %v", oldPath, newPath, err)
		return
	}
	if err := os.Remove(oldPath); err != nil {
		log.Printf("config: migration: copied config but could not remove old file %s: %v", oldPath, err)
	}
	log.Printf("config: migrated config from %s to %s", oldPath, newPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return err
	}
	return out.Close()
}
