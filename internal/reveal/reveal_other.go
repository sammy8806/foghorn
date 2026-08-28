//go:build !darwin && !windows

package reveal

import (
	"os/exec"
	"path/filepath"
)

// InFileManager opens the containing directory: xdg-open has no equivalent of
// Finder's reveal, and pointing it at the file would launch an editor instead.
func InFileManager(path string) error {
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
