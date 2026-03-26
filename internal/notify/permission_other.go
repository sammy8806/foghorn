//go:build !darwin

package notify

func NotificationPermissionStatus() string {
	return "unsupported"
}
