//go:build darwin

package notify

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

static char *foghornNSErrorString(NSError *error) {
	if (error == nil) {
		return NULL;
	}
	const char *domain = error.domain != nil ? error.domain.UTF8String : "";
	const char *description = error.localizedDescription != nil ? error.localizedDescription.UTF8String : "";
	int code = (int)error.code;
	size_t len = snprintf(NULL, 0, "%s\x1f%d\x1f%s", domain, code, description);
	char *buffer = malloc(len + 1);
	if (buffer == NULL) {
		return strdup(description);
	}
	snprintf(buffer, len + 1, "%s\x1f%d\x1f%s", domain, code, description);
	return buffer;
}

static char *foghornSendUserNotification(const char *title, const char *body) {
	if (@available(macOS 10.14, *)) {
		__block UNAuthorizationStatus status = UNAuthorizationStatusNotDetermined;
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);

		UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
		[center getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *settings) {
			status = settings.authorizationStatus;
			dispatch_semaphore_signal(sem);
		}];
		dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);

		if (status == UNAuthorizationStatusNotDetermined) {
			__block BOOL granted = NO;
			__block NSError *authError = nil;
			dispatch_semaphore_t authSem = dispatch_semaphore_create(0);

			[center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound | UNAuthorizationOptionBadge)
				completionHandler:^(BOOL didGrant, NSError * _Nullable error) {
					granted = didGrant;
					authError = error;
					dispatch_semaphore_signal(authSem);
				}];
			dispatch_semaphore_wait(authSem, DISPATCH_TIME_FOREVER);

			if (authError != nil) {
				return foghornNSErrorString(authError);
			}
			if (!granted) {
				return strdup("permission_denied");
			}
		} else if (status == UNAuthorizationStatusDenied) {
			return strdup("permission_denied");
		}

		NSString *identifier = [NSString stringWithFormat:@"foghorn-%f", [NSDate timeIntervalSinceReferenceDate]];
		UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
		content.title = [NSString stringWithUTF8String:title];
		content.body = [NSString stringWithUTF8String:body];
		content.sound = [UNNotificationSound defaultSound];

		UNTimeIntervalNotificationTrigger *trigger = [UNTimeIntervalNotificationTrigger triggerWithTimeInterval:0.1 repeats:NO];
		UNNotificationRequest *request = [UNNotificationRequest requestWithIdentifier:identifier content:content trigger:trigger];

		__block NSError *deliveryError = nil;
		dispatch_semaphore_t sendSem = dispatch_semaphore_create(0);
		[center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
			deliveryError = error;
			dispatch_semaphore_signal(sendSem);
		}];
		dispatch_semaphore_wait(sendSem, DISPATCH_TIME_FOREVER);

		if (deliveryError != nil) {
			return foghornNSErrorString(deliveryError);
		}

		return NULL;
	}

	return strdup("unsupported_legacy");
}
*/
import "C"

import (
	"os"
	"strconv"
	"strings"
	"unsafe"
)

func defaultSend(title, body string) error {
	cTitle := C.CString(title)
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cBody))

	errPtr := C.foghornSendUserNotification(cTitle, cBody)
	if errPtr != nil {
		defer C.free(unsafe.Pointer(errPtr))
		message := C.GoString(errPtr)
		parts := strings.SplitN(message, "\x1f", 3)
		if len(parts) == 3 {
			code, _ := strconv.Atoi(parts[1])
			executablePath, _ := os.Executable()
			return formatMacNotificationError(parts[0], code, parts[2], bundleIdentifier(), executablePath)
		}
		return formatMacNotificationError("", 0, message, bundleIdentifier(), "")
	}
	return nil
}
