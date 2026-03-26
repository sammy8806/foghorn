package notify

import (
	"fmt"
	"path/filepath"
	"strings"
)

func formatMacNotificationError(domain string, code int, description string, bundleID string, executablePath string) error {
	description = strings.TrimSpace(description)
	if domain == "UNErrorDomain" && code == 1 {
		message := "notifications are not allowed for this app"
		if bundleID != "" {
			message = fmt.Sprintf("%s (%s)", message, bundleID)
		}
		if !runningInsideAppBundle(executablePath) {
			message += "; macOS often blocks notifications for dev/unbundled binaries, so build and launch the .app bundle instead of running via wails dev"
		} else {
			message += "; allow Foghorn in System Settings > Notifications"
		}
		if description != "" && description != "The operation couldn’t be completed. (UNErrorDomain error 1.)" {
			message += fmt.Sprintf(": %s", description)
		}
		return fmt.Errorf(message)
	}

	if domain != "" && code != 0 && description != "" {
		return fmt.Errorf("%s error %d: %s", domain, code, description)
	}
	if domain != "" && code != 0 {
		return fmt.Errorf("%s error %d", domain, code)
	}
	if description != "" {
		return fmt.Errorf(description)
	}
	return fmt.Errorf("notification failed")
}

func runningInsideAppBundle(executablePath string) bool {
	if executablePath == "" {
		return false
	}
	path := filepath.Clean(executablePath)
	return strings.Contains(path, ".app/Contents/MacOS/")
}
