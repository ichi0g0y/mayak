#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import "browser_view_darwin.h"
extern void mayakBrowserEvent(uintptr_t, char *, char *, int, int, int);
static BOOL validURL(NSURL *url){return url.host.length>0 && ([url.scheme isEqualToString:@"https"]||[url.scheme isEqualToString:@"http"]) && ![url.host.lowercaseString isEqualToString:@"wails.localhost"] && !url.user && !url.password;}
@interface MayakBrowserTab : NSObject <WKNavigationDelegate,WKUIDelegate>
@property(retain) WKWebView *view;
@property(retain) NSArray *constraints;
@property(assign) uintptr_t handle;
@end
@implementation MayakBrowserTab
-(void)notify:(BOOL)popup url:(NSURL*)url{
 if(url)mayakBrowserEvent(self.handle,(char*)url.absoluteString.UTF8String,(char*)(self.view.title?:@"").UTF8String,self.view.canGoBack,self.view.canGoForward,popup);
}
-(void)observeValueForKeyPath:(NSString*)path ofObject:(id)object change:(NSDictionary*)change context:(void*)context{[self notify:NO url:self.view.URL];}
-(void)webView:(WKWebView*)view decidePolicyForNavigationAction:(WKNavigationAction*)action decisionHandler:(void (^)(WKNavigationActionPolicy))handler{
 handler(validURL(action.request.URL)?WKNavigationActionPolicyAllow:WKNavigationActionPolicyCancel);
}
-(WKWebView*)webView:(WKWebView*)view createWebViewWithConfiguration:(WKWebViewConfiguration*)configuration forNavigationAction:(WKNavigationAction*)action windowFeatures:(WKWindowFeatures*)features{
 if(validURL(action.request.URL)&&action.navigationType==WKNavigationTypeLinkActivated)[self notify:YES url:action.request.URL];return nil;
}
-(void)webView:(WKWebView*)view requestMediaCapturePermissionForOrigin:(WKSecurityOrigin*)origin initiatedByFrame:(WKFrameInfo*)frame type:(WKMediaCaptureType)type decisionHandler:(void (^)(WKPermissionDecision))handler API_AVAILABLE(macos(12.0)){handler(WKPermissionDecisionDeny);}
-(void)dealloc{[_constraints release];[_view release];[super dealloc];}
@end
static void onMain(void (^block)(void)){if([NSThread isMainThread])block();else dispatch_sync(dispatch_get_main_queue(),block);}
void *rl_browser_new(void *ptr,uintptr_t handle){
 __block MayakBrowserTab *tab=nil;
 onMain(^{
  NSWindow *window=(NSWindow*)ptr;
 NSView *content=window.contentView;
  WKWebViewConfiguration *config=[[WKWebViewConfiguration alloc]init];
  // A new content controller contains no Wails script/message handlers.
  config.websiteDataStore=[WKWebsiteDataStore defaultDataStore];
  WKWebView *view=[[WKWebView alloc]initWithFrame:NSZeroRect configuration:config];[config release];
  tab=[[MayakBrowserTab alloc]init];tab.view=view;tab.handle=handle;view.navigationDelegate=tab;view.UIDelegate=tab;
  view.hidden=YES;view.translatesAutoresizingMaskIntoConstraints=NO;
  [content addSubview:view];
  tab.constraints=@[[view.leadingAnchor constraintEqualToAnchor:content.leadingAnchor], [view.topAnchor constraintEqualToAnchor:content.topAnchor], [view.trailingAnchor constraintEqualToAnchor:content.trailingAnchor], [view.bottomAnchor constraintEqualToAnchor:content.bottomAnchor]];
  [NSLayoutConstraint activateConstraints:tab.constraints];
  for(NSString *key in @[@"URL",@"title",@"canGoBack",@"canGoForward"])[view addObserver:tab forKeyPath:key options:0 context:NULL];
  [view release];
 });return tab;
}
void rl_browser_action(void *ptr,const char *rawCommand,const char *rawURL,int left,int top,int right,int bottom){
 NSString *command=[NSString stringWithUTF8String:rawCommand],*url=[NSString stringWithUTF8String:rawURL];
 onMain(^{
  MayakBrowserTab *tab=(MayakBrowserTab*)ptr;WKWebView *view=tab.view;
  if([command isEqualToString:@"show"]){((NSLayoutConstraint*)tab.constraints[0]).constant=left;((NSLayoutConstraint*)tab.constraints[1]).constant=top;((NSLayoutConstraint*)tab.constraints[2]).constant=-right;((NSLayoutConstraint*)tab.constraints[3]).constant=-bottom;view.hidden=NO;}
  else if([command isEqualToString:@"hide"])view.hidden=YES;
  else if([command isEqualToString:@"navigate"]&&validURL([NSURL URLWithString:url]))[view loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:url]]];
  else if([command isEqualToString:@"back"])[view goBack];
  else if([command isEqualToString:@"forward"])[view goForward];
  else if([command isEqualToString:@"reload"])[view reload];
  else if([command isEqualToString:@"close"]){
   view.navigationDelegate=nil;view.UIDelegate=nil;
   for(NSString *key in @[@"URL",@"title",@"canGoBack",@"canGoForward"])[view removeObserver:tab forKeyPath:key];
   [view stopLoading];[NSLayoutConstraint deactivateConstraints:tab.constraints];[view removeFromSuperview];[tab release];
  }
 });
}
