package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrate_MovesFileToNewLocation(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old", "config.yaml")
	newPath := filepath.Join(dir, "new", "config.yaml")

	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("sources: []"), 0o600); err != nil {
		t.Fatal(err)
	}

	Migrate(oldPath, newPath)

	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("expected config at new path: %v", err)
	}
	if _, err := os.Stat(oldPath); err == nil {
		t.Error("expected old config to be removed after migration")
	}
}

func TestMigrate_PreservesFileContent(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old", "config.yaml")
	newPath := filepath.Join(dir, "new", "config.yaml")

	content := "sources:\n  - name: test\n    type: alertmanager\n    url: http://localhost:9093\n"
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	Migrate(oldPath, newPath)

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("could not read migrated config: %v", err)
	}
	if string(got) != content {
		t.Errorf("content mismatch after migration: got %q, want %q", got, content)
	}
}

func TestMigrate_SkipsWhenNewPathAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old", "config.yaml")
	newPath := filepath.Join(dir, "new", "config.yaml")

	for _, p := range []string{oldPath, newPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(oldPath, []byte("old content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new content"), 0o600); err != nil {
		t.Fatal(err)
	}

	Migrate(oldPath, newPath)

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Errorf("expected new config to be untouched, got %q", got)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Error("expected old config to remain when new config already exists")
	}
}

func TestMigrate_NoopWhenOldPathMissing(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old", "config.yaml")
	newPath := filepath.Join(dir, "new", "config.yaml")

	Migrate(oldPath, newPath) // must not panic or error

	if _, err := os.Stat(newPath); err == nil {
		t.Error("expected new config not to be created when old config is missing")
	}
}

func TestMigrate_NoopWhenPathsIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	Migrate(path, path)

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to remain after no-op migration: %v", err)
	}
}
