//go:build !darwin

package notify

import "github.com/gen2brain/beeep"

func defaultSend(title, body string) error {
	beeep.AppName = "Foghorn"
	return beeep.Notify(title, body, "")
}
