//go:build !darwin

package notify

func NotificationPermissionStatus() string {
	return "unsupported"
}

func RequestNotificationPermission() (string, error) {
	return "unsupported", nil
}
