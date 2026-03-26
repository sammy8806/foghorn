//go:build darwin

package notify

import "os/exec"

func OpenNotificationSettings() error {
	if err := exec.Command("open", "x-apple.systempreferences:com.apple.preference.notifications").Start(); err == nil {
		return nil
	}
	return exec.Command("open", "/System/Library/PreferencePanes/Notifications.prefPane").Start()
}
