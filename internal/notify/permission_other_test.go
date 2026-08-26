//go:build !darwin

package notify

import "testing"

func TestRequestNotificationPermissionUnsupported(t *testing.T) {
	status, err := RequestNotificationPermission()
	if err != nil {
		t.Fatalf("RequestNotificationPermission() error: %v", err)
	}
	if status != "unsupported" {
		t.Fatalf("RequestNotificationPermission() = %q, want %q", status, "unsupported")
	}
}
