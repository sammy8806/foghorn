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
				return strdup(authError.localizedDescription.UTF8String);
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
			return strdup(deliveryError.localizedDescription.UTF8String);
		}

		return NULL;
	}

	return strdup("unsupported_legacy");
}
*/
import "C"

import (
	"fmt"
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
		return fmt.Errorf(C.GoString(errPtr))
	}
	return nil
}
