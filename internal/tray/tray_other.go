//go:build !darwin

package tray

import "log"

type noopTray struct{}

func newPlatformTray(_ *Manager) platformTray {
	log.Println("tray: native tray is only implemented on macOS")
	return &noopTray{}
}

func (n *noopTray) update(_ []byte, _ string) error {
	return nil
}

func (n *noopTray) dispose() {}
