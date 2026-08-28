//go:build windows

package reveal

import (
	"os/exec"
	"path/filepath"
)

// InFileManager opens the broken config's containing directory in Explorer.
func InFileManager(path string) error {
	return exec.Command("explorer.exe", filepath.Dir(path)).Start()
}
