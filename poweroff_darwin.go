//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

extern void foghornHandlePowerOff(void);

void foghornObservePowerOff(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[[[NSWorkspace sharedWorkspace] notificationCenter]
			addObserverForName:NSWorkspaceWillPowerOffNotification
			object:nil
			queue:[NSOperationQueue mainQueue]
			usingBlock:^(NSNotification *note) {
				foghornHandlePowerOff();
			}];
	});
}
*/
import "C"

// powerOffHandler is invoked from foghornHandlePowerOff (see
// poweroff_callback_darwin.go) when macOS announces a logout/shutdown.
var powerOffHandler func()

// watchSystemPowerOff quits the app when macOS begins a logout, restart or
// shutdown. Wails' AppDelegate answers the system's terminate request with
// NSTerminateCancel and routes it through OnBeforeClose, which for Foghorn
// hides the window instead of quitting — macOS then reports that Foghorn
// interrupted the shutdown. NSWorkspaceWillPowerOffNotification is posted
// before apps are asked to terminate, so quitting here lets shutdown proceed.
func watchSystemPowerOff(handler func()) {
	powerOffHandler = handler
	C.foghornObservePowerOff()
}
