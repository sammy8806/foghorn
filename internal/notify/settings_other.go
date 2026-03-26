//go:build !darwin

package notify

import "fmt"

func OpenNotificationSettings() error {
	return fmt.Errorf("notification settings shortcut is only available on macOS")
}
