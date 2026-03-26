//go:build !darwin && (!linux || !linux_tray)

package tray

import "log"

type noopTray struct{}

func Supported() bool {
	return false
}

func StartHiddenByDefault() bool {
	return false
}

func newPlatformTray(_ *Manager) platformTray {
	log.Println("tray: native tray is only implemented on macOS")
	return &noopTray{}
}

func (n *noopTray) update(_ []byte, _ string) error {
	return nil
}

func (n *noopTray) dispose() {}
