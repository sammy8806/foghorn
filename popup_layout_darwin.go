//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>

static void foghornLayoutPopupWindow(int width, int height, int rightMargin, int topMargin, int bottomMargin) {
	dispatch_sync(dispatch_get_main_queue(), ^{
		NSWindow *window = [NSApp mainWindow];
		if (window == nil) {
			window = [NSApp keyWindow];
		}
		if (window == nil) {
			return;
		}

		NSScreen *screen = [window screen];
		if (screen == nil) {
			screen = [NSScreen mainScreen];
		}
		if (screen == nil) {
			return;
		}

		NSRect visible = [screen visibleFrame];
		CGFloat nextWidth = MIN((CGFloat)width, MAX((CGFloat)240, visible.size.width - rightMargin));
		CGFloat nextHeight = MIN((CGFloat)height, MAX((CGFloat)200, visible.size.height - topMargin - bottomMargin));
		CGFloat x = visible.origin.x + visible.size.width - nextWidth - rightMargin;
		CGFloat y = visible.origin.y + visible.size.height - nextHeight - topMargin;

		NSRect frame = [window frame];
		frame.origin.x = x;
		frame.origin.y = y;
		frame.size.width = nextWidth;
		frame.size.height = nextHeight;

		[window setFrame:frame display:YES animate:NO];
	});
}
*/
import "C"

func layoutPopupWindow(width, height, rightMargin, topMargin, bottomMargin int) {
	C.foghornLayoutPopupWindow(
		C.int(width),
		C.int(height),
		C.int(rightMargin),
		C.int(topMargin),
		C.int(bottomMargin),
	)
}
