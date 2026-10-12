package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// WebKit holds back a page that cannot be seen (a window behind others,
// minimised or hidden in the tray), App Nap or not (NSAppSleepDisabled
// stops App Nap only). The shell must still hear its Host there: a task
// the Host recognises should be open when the window comes back. So its
// web view keeps running when it is not seen (macOS 14 and later). The
// shell's web view is the first one in the window; the tabs' pages, added
// after it (internal/browserview), are left as they are.
static void mayakKeepShellRunning(void *ptr) {
	NSWindow *window = (__bridge NSWindow *)ptr;
	dispatch_async(dispatch_get_main_queue(), ^{
		if (@available(macOS 14.0, *)) {
			for (NSView *view in window.contentView.subviews) {
				if (![view isKindOfClass:[WKWebView class]]) continue;
				((WKWebView *)view).configuration.preferences.inactiveSchedulingPolicy = WKInactiveSchedulingPolicyNone;
				return;
			}
		}
	});
}
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// keepShellRunning keeps the shell's page running while the window is not
// seen (see mayakKeepShellRunning).
func keepShellRunning(window *application.WebviewWindow) {
	if ptr := window.NativeWindow(); ptr != nil {
		C.mayakKeepShellRunning(ptr)
	}
}
