//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation

#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

static char *foghornBundleIdentifier() {
	NSString *bundleID = [[NSBundle mainBundle] bundleIdentifier];
	if (bundleID == nil || bundleID.length == 0) {
		return NULL;
	}
	return strdup(bundleID.UTF8String);
}
*/
import "C"

import (
	"fmt"
	"os/exec"
	"unsafe"
)

func OpenNotificationSettings() error {
	if bundleID := bundleIdentifier(); bundleID != "" {
		target := fmt.Sprintf("x-apple.systempreferences:com.apple.preference.notifications?id=%s", bundleID)
		if err := exec.Command("open", target).Start(); err == nil {
			return nil
		}
	}
	if err := exec.Command("open", "x-apple.systempreferences:com.apple.preference.notifications").Start(); err == nil {
		return nil
	}
	return exec.Command("open", "/System/Library/PreferencePanes/Notifications.prefPane").Start()
}

func bundleIdentifier() string {
	value := C.foghornBundleIdentifier()
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
