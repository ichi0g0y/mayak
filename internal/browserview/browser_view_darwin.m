#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import "browser_view_darwin.h"
extern void mayakBrowserEvent(uintptr_t, char *, char *, int, int, int);
static BOOL validURL(NSURL *url){return url.host.length>0 && ([url.scheme isEqualToString:@"https"]||[url.scheme isEqualToString:@"http"]) && ![url.host.lowercaseString isEqualToString:@"wails.localhost"] && !url.user && !url.password;}
// A window.open with a size or position is a dialog that reports back to its
// opener (Google sign-in posts its result to window.opener and closes), as on
// Windows (go-webview2 browser_tabs.go): it opens in a small window of its own
// with the web view WebKit asks for, made with the configuration it gives, so
// that the two can talk; the page closing it closes the window. A link to a
// new window (target=_blank, no size) opens in a tab instead. WebKit already
// drops a window.open a page makes without a click.
@interface MayakPopup : NSObject <WKUIDelegate,NSWindowDelegate>
@property(retain) NSWindow *window;
@property(retain) WKWebView *view;
@end
static NSMutableArray *mayakPopups;
static BOOL mayakDialog(WKWindowFeatures *features){return features.width||features.height||features.x||features.y;}
static WKWebView *mayakOpenPopup(WKWebViewConfiguration *configuration,WKWindowFeatures *features,NSWindow *parent);
@implementation MayakPopup
-(WKWebView*)webView:(WKWebView*)view createWebViewWithConfiguration:(WKWebViewConfiguration*)configuration forNavigationAction:(WKNavigationAction*)action windowFeatures:(WKWindowFeatures*)features{
 return mayakDialog(features)?mayakOpenPopup(configuration,features,self.window):nil;
}
-(void)webViewDidClose:(WKWebView*)view{[self.window close];}
-(void)windowWillClose:(NSNotification*)note{
 self.view.UIDelegate=nil;self.window.delegate=nil;[self.view stopLoading];
 // Let go once AppKit is done closing the window.
 MayakPopup *popup=self;dispatch_async(dispatch_get_main_queue(),^{[mayakPopups removeObject:popup];});
}
-(void)dealloc{[_view release];[_window release];[super dealloc];}
@end
static WKWebView *mayakOpenPopup(WKWebViewConfiguration *configuration,WKWindowFeatures *features,NSWindow *parent){
 CGFloat width=features.width?features.width.doubleValue:520,height=features.height?features.height.doubleValue:680;
 width=MAX(360,MIN(width,1400));height=MAX(420,MIN(height,1000));
 NSRect frame=NSMakeRect(0,0,width,height);
 NSWindow *window=[[NSWindow alloc]initWithContentRect:frame styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
 window.releasedWhenClosed=NO;window.title=@"MAYAK";
 WKWebView *view=[[WKWebView alloc]initWithFrame:frame configuration:configuration];
 MayakPopup *popup=[[MayakPopup alloc]init];popup.window=window;popup.view=view;
 view.UIDelegate=popup;window.delegate=popup;window.contentView=view;
 if(parent){NSRect p=parent.frame;[window setFrameOrigin:NSMakePoint(NSMidX(p)-width/2,NSMidY(p)-height/2)];}else [window center];
 [window makeKeyAndOrderFront:nil];
 if(!mayakPopups)mayakPopups=[[NSMutableArray alloc]init];
 [mayakPopups addObject:popup];[popup release];[view release];[window release];
 return view;
}
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
 if(mayakDialog(features))return mayakOpenPopup(configuration,features,view.window);
 if(validURL(action.request.URL))[self notify:YES url:action.request.URL];
 return nil;
}
-(void)webView:(WKWebView*)view requestMediaCapturePermissionForOrigin:(WKSecurityOrigin*)origin initiatedByFrame:(WKFrameInfo*)frame type:(WKMediaCaptureType)type decisionHandler:(void (^)(WKPermissionDecision))handler API_AVAILABLE(macos(12.0)){handler(WKPermissionDecisionDeny);}
-(void)dealloc{[_constraints release];[_view release];[super dealloc];}
@end
// The ad blocker (adblock/webkit.go, view_darwin.go watchRules): one WebKit
// content blocker every tab takes (and the popups they open, which share the
// tab's content controller), made from the filter lists and kept by WebKit
// under an identifier with their version, so a start with the same lists
// finds it compiled. Should WebKit refuse the element hiding, the network
// rules go alone.
static WKContentRuleList *mayakRules;
static BOOL mayakRulesOn=YES;
static NSMutableArray *mayakTabs;
static void mayakApplyRules(WKWebView *view){
 WKUserContentController *controller=view.configuration.userContentController;
 [controller removeAllContentRuleLists];
 if(mayakRules&&mayakRulesOn)[controller addContentRuleList:mayakRules];
}
static void mayakApplyAll(void){for(MayakBrowserTab *tab in mayakTabs)mayakApplyRules(tab.view);}
static void mayakSetRules(WKContentRuleList *list){
 dispatch_async(dispatch_get_main_queue(),^{[mayakRules release];mayakRules=[list retain];mayakApplyAll();});
}
static void mayakForgetOthers(WKContentRuleListStore *store,NSString *keep){
 [store getAvailableContentRuleListIdentifiers:^(NSArray<NSString*> *identifiers){
  for(NSString *identifier in identifiers)if([identifier hasPrefix:@"mayak-adblock-"]&&![identifier isEqualToString:keep])[store removeContentRuleListForIdentifier:identifier completionHandler:^(NSError *error){}];
 }];
}
void rl_browser_rules(const char *all,const char *network,const char *version){
 NSString *allText=[NSString stringWithUTF8String:all],*networkText=[NSString stringWithUTF8String:network];
 NSString *identifier=[NSString stringWithFormat:@"mayak-adblock-%s",version];
 dispatch_async(dispatch_get_main_queue(),^{
  WKContentRuleListStore *store=[WKContentRuleListStore defaultStore];
  [store lookUpContentRuleListForIdentifier:identifier completionHandler:^(WKContentRuleList *found,NSError *lookUpError){
   if(found){mayakSetRules(found);return;}
   [store compileContentRuleListForIdentifier:identifier encodedContentRuleList:allText completionHandler:^(WKContentRuleList *list,NSError *error){
    if(list){mayakSetRules(list);mayakForgetOthers(store,identifier);return;}
    NSLog(@"MAYAK: content blocker refused (%@); network rules only",error);
    [store compileContentRuleListForIdentifier:identifier encodedContentRuleList:networkText completionHandler:^(WKContentRuleList *network,NSError *networkError){
     if(network){mayakSetRules(network);mayakForgetOthers(store,identifier);}
     else NSLog(@"MAYAK: content blocker failed: %@",networkError);
    }];
   }];
  }];
 });
}
void rl_browser_rules_enabled(int on){
 dispatch_async(dispatch_get_main_queue(),^{if(mayakRulesOn!=(on!=0)){mayakRulesOn=on!=0;mayakApplyAll();}});
}
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
  if(!mayakTabs)mayakTabs=[[NSMutableArray alloc]init];
  [mayakTabs addObject:tab];mayakApplyRules(view);
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
   [view stopLoading];[NSLayoutConstraint deactivateConstraints:tab.constraints];[view removeFromSuperview];[mayakTabs removeObject:tab];[tab release];
  }
 });
}
