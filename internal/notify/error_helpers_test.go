package notify

import (
	"strings"
	"testing"
)

func TestFormatMacNotificationErrorForUnbundledApp(t *testing.T) {
	err := formatMacNotificationError(
		"UNErrorDomain",
		1,
		"The operation couldn’t be completed. (UNErrorDomain error 1.)",
		"com.wails.foghorn",
		"/tmp/go-build1234/b001/exe/foghorn",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "notifications are not allowed for this app") {
		t.Fatalf("unexpected message: %q", msg)
	}
	if !strings.Contains(msg, "build and launch the .app bundle") {
		t.Fatalf("missing unbundled hint: %q", msg)
	}
}

func TestFormatMacNotificationErrorForBundledApp(t *testing.T) {
	err := formatMacNotificationError(
		"UNErrorDomain",
		1,
		"",
		"com.wails.foghorn",
		"/Applications/Foghorn.app/Contents/MacOS/foghorn",
	)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "allow Foghorn in System Settings > Notifications") {
		t.Fatalf("missing settings hint: %q", msg)
	}
}

func TestRunningInsideAppBundle(t *testing.T) {
	if !runningInsideAppBundle("/Applications/Foghorn.app/Contents/MacOS/foghorn") {
		t.Fatal("expected bundled executable to be detected")
	}
	if runningInsideAppBundle("/tmp/go-build1234/b001/exe/foghorn") {
		t.Fatal("expected unbundled executable to be detected")
	}
}
