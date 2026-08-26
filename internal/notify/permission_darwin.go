//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>
#include <string.h>

static const char *foghornNotificationPermissionStatus() {
	if (@available(macOS 10.14, *)) {
		__block NSInteger status = -1;
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);

		[[UNUserNotificationCenter currentNotificationCenter]
			getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
				status = settings.authorizationStatus;
				dispatch_semaphore_signal(sem);
			}];

		dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

		switch (status) {
		case 2:
			return "authorized";
		case 1:
			return "denied";
		case 0:
			return "not_determined";
#ifdef UNAuthorizationStatusProvisional
		case UNAuthorizationStatusProvisional:
			return "provisional";
#endif
#ifdef UNAuthorizationStatusEphemeral
		case UNAuthorizationStatusEphemeral:
			return "ephemeral";
#endif
		default:
			return "unknown";
		}
	}

	return "unsupported_legacy";
}

static char *foghornRequestNotificationPermission() {
	if (@available(macOS 10.14, *)) {
		__block NSError *authError = nil;
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);

		[[UNUserNotificationCenter currentNotificationCenter]
			requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge)
			completionHandler:^(BOOL granted, NSError * _Nullable error) {
				(void)granted;
				authError = error;
				dispatch_semaphore_signal(sem);
			}];
		dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

		if (authError != nil) {
			NSString *description = authError.localizedDescription ?: @"notification authorization failed";
			NSString *result = [@"error\x1f" stringByAppendingString:description];
			return strdup(result.UTF8String);
		}

		// Read the stored status instead of relying only on the completion
		// handler's boolean, which may also be false while authorization remains
		// undetermined.
		return strdup(foghornNotificationPermissionStatus());
	}

	return strdup("unsupported_legacy");
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

func NotificationPermissionStatus() string {
	return C.GoString(C.foghornNotificationPermissionStatus())
}

// RequestNotificationPermission asks macOS to authorize the alert, sound, and
// badge interactions used by Foghorn, then returns the resulting status.
func RequestNotificationPermission() (string, error) {
	value := C.foghornRequestNotificationPermission()
	if value == nil {
		return "", fmt.Errorf("request notification permission: no result returned")
	}
	defer C.free(unsafe.Pointer(value))

	result := C.GoString(value)
	if description, ok := strings.CutPrefix(result, "error\x1f"); ok {
		return "", fmt.Errorf("request notification permission: %s", description)
	}
	return result, nil
}
