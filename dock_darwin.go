//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include <stdbool.h>

static void foghornSetDockIconVisible(bool visible) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:visible ? NSApplicationActivationPolicyRegular : NSApplicationActivationPolicyAccessory];
		if (visible) {
			[NSApp activateIgnoringOtherApps:YES];
		}
	});
}
*/
import "C"

func setDockIconVisible(visible bool) {
	C.foghornSetDockIconVisible(C.bool(visible))
}
