package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// The shell's toolbar is 48pt high and reaches the window's top edge (the
// title bar is transparent and the page is under it). macOS puts its red,
// yellow and green buttons in a 28pt title bar, so they would sit above the
// middle of the toolbar. This makes the title bar area 48pt high and puts the
// buttons at its middle, the first one centred 22pt from the left edge, as
// the shell's own buttons were (style.css, --lights).
static const CGFloat mayakBarHeight = 48, mayakFirstCentre = 22;

static void mayakPlaceButtons(NSWindow *window) {
	// In full screen the buttons are in the menu bar that slides down.
	if (window.styleMask & NSWindowStyleMaskFullScreen) return;
	NSButton *close = [window standardWindowButton:NSWindowCloseButton];
	NSButton *minimise = [window standardWindowButton:NSWindowMiniaturizeButton];
	NSButton *zoom = [window standardWindowButton:NSWindowZoomButton];
	NSView *bar = close.superview.superview;
	if (close == nil || minimise == nil || zoom == nil || bar == nil) return;
	NSRect frame = bar.frame;
	frame.size.height = mayakBarHeight;
	frame.origin.y = NSHeight(window.frame) - mayakBarHeight;
	bar.frame = frame;
	CGFloat step = NSMinX(minimise.frame) - NSMinX(close.frame);
	NSArray<NSButton *> *buttons = @[close, minimise, zoom];
	for (NSUInteger i = 0; i < buttons.count; i++) {
		NSRect button = buttons[i].frame;
		button.origin.x = mayakFirstCentre - NSWidth(button) / 2 + i * step;
		button.origin.y = (NSHeight(close.superview.frame) - NSHeight(button)) / 2;
		[buttons[i] setFrameOrigin:button.origin];
	}
}

// AppKit lays the title bar out again when the window changes size, leaves
// full screen or becomes active, so the buttons are put back each time.
static void mayakKeepButtonsPlaced(void *ptr) {
	NSWindow *window = (__bridge NSWindow *)ptr;
	dispatch_async(dispatch_get_main_queue(), ^{
		mayakPlaceButtons(window);
		NSNotificationCenter *centre = [NSNotificationCenter defaultCenter];
		for (NSNotificationName name in @[NSWindowDidResizeNotification, NSWindowDidExitFullScreenNotification, NSWindowDidBecomeKeyNotification, NSWindowDidResignKeyNotification, NSWindowDidChangeBackingPropertiesNotification]) {
			[centre addObserverForName:name object:window queue:nil usingBlock:^(NSNotification *note) {
				mayakPlaceButtons(window);
			}];
		}
	});
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// macWindow is the main window's macOS look: the system's own red, yellow
// and green buttons over a page that reaches the top edge, with no title
// and no title bar to see (the shell's layout is the same as frameless).
var macWindow = application.MacWindow{TitleBar: application.MacTitleBarHidden}

// keepWindowButtonsPlaced puts the window buttons at the middle of the
// shell's toolbar (see mayakPlaceButtons) and keeps them there.
func keepWindowButtonsPlaced(window *application.WebviewWindow) {
	if ptr := window.NativeWindow(); ptr != nil {
		C.mayakKeepButtonsPlaced(ptr)
	}
}
