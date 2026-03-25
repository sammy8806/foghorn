#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include <stdint.h>

extern void trayHandleClick(uintptr_t handle);
extern void trayHandleQuit(uintptr_t handle);

@interface FoghornTrayTarget : NSObject
@property(nonatomic, assign) uintptr_t handle;
@property(nonatomic, strong) NSStatusItem *statusItem;
@property(nonatomic, strong) NSMenu *menu;
@end

@implementation FoghornTrayTarget

- (void)toggleWindow:(id)sender {
	(void)sender;
	trayHandleClick(self.handle);
}

- (void)quitApp:(id)sender {
	(void)sender;
	trayHandleQuit(self.handle);
}

- (void)statusItemClicked:(id)sender {
	(void)sender;
	NSEvent *event = [NSApp currentEvent];
	if (event.type == NSEventTypeRightMouseUp || (event.modifierFlags & NSEventModifierFlagControl) == NSEventModifierFlagControl) {
		[NSMenu popUpContextMenu:self.menu withEvent:event forView:[self.statusItem button]];
		return;
	}
	trayHandleClick(self.handle);
}

@end

static void foghornRunOnMainSync(dispatch_block_t block) {
	if ([NSThread isMainThread]) {
		block();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), block);
}

void *foghornTrayCreate(uintptr_t handle) {
	__block FoghornTrayTarget *target = nil;
	foghornRunOnMainSync(^{
		target = [FoghornTrayTarget new];
		target.handle = handle;
		target.statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
		target.menu = [[NSMenu alloc] initWithTitle:@"Foghorn"];

		NSMenuItem *toggle = [[NSMenuItem alloc] initWithTitle:@"Show or Hide Window" action:@selector(toggleWindow:) keyEquivalent:@""];
		[toggle setTarget:target];
		[target.menu addItem:toggle];
		[target.menu addItem:[NSMenuItem separatorItem]];

		NSMenuItem *quit = [[NSMenuItem alloc] initWithTitle:@"Quit Foghorn" action:@selector(quitApp:) keyEquivalent:@""];
		[quit setTarget:target];
		[target.menu addItem:quit];

		NSStatusBarButton *button = [target.statusItem button];
		[button setTarget:target];
		[button setAction:@selector(statusItemClicked:)];
		[button sendActionOn:NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp];
		[button setImagePosition:NSImageOnly];
		[button setToolTip:@"Foghorn"];
	});
	return (__bridge_retained void *)target;
}

void foghornTrayUpdate(void *targetRef, void *bytes, int length, const char *tooltip) {
	if (targetRef == nil) {
		return;
	}
	FoghornTrayTarget *target = (__bridge FoghornTrayTarget *)targetRef;
	NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
	NSString *tip = [NSString stringWithUTF8String:tooltip];
	foghornRunOnMainSync(^{
		NSImage *image = [[NSImage alloc] initWithData:data];
		[image setSize:NSMakeSize(18, 18)];
		[target.statusItem.button setImage:image];
		[target.statusItem.button setToolTip:tip];
	});
}

void foghornTrayDispose(void *targetRef) {
	if (targetRef == nil) {
		return;
	}
	FoghornTrayTarget *target = (__bridge_transfer FoghornTrayTarget *)targetRef;
	foghornRunOnMainSync(^{
		[[NSStatusBar systemStatusBar] removeStatusItem:target.statusItem];
	});
}
