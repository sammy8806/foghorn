//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>
#include <string.h>

static void foghornLayoutPopupWindow(int width, int height, int horizontalMargin, int topMargin, int bottomMargin, char *position) {
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
		BOOL alignLeft = strcmp(position, "top_left") == 0 || strcmp(position, "bottom_left") == 0;
		BOOL alignBottom = strcmp(position, "bottom_left") == 0 || strcmp(position, "bottom_right") == 0;
		CGFloat nextWidth = MIN((CGFloat)width, MAX((CGFloat)240, visible.size.width - horizontalMargin));
		CGFloat nextHeight = MIN((CGFloat)height, MAX((CGFloat)200, visible.size.height - topMargin - bottomMargin));
		CGFloat x = alignLeft
			? visible.origin.x + horizontalMargin
			: visible.origin.x + visible.size.width - nextWidth - horizontalMargin;
		CGFloat y = alignBottom
			? visible.origin.y + bottomMargin
			: visible.origin.y + visible.size.height - nextHeight - topMargin;

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
import "unsafe"

func layoutPopupWindow(width, height, horizontalMargin, topMargin, bottomMargin int, position string) {
	cPosition := C.CString(position)
	defer C.free(unsafe.Pointer(cPosition))
	C.foghornLayoutPopupWindow(
		C.int(width),
		C.int(height),
		C.int(horizontalMargin),
		C.int(topMargin),
		C.int(bottomMargin),
		cPosition,
	)
}
