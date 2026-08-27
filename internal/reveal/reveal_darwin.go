//go:build darwin

package reveal

import "os/exec"

// InFileManager selects the file in Finder rather than opening it, so a broken
// config is not handed to whatever app claims .yaml.
func InFileManager(path string) error {
	return exec.Command("open", "-R", path).Start()
}
