//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#import <dispatch/dispatch.h>

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
*/
import "C"

func NotificationPermissionStatus() string {
	return C.GoString(C.foghornNotificationPermissionStatus())
}
