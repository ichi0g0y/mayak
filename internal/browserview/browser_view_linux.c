#include "browser_view_linux.h"
#include <string.h>
extern void mayakBrowserEvent(uintptr_t, char *, char *, int, int, int);
static gboolean valid_uri(const char *uri) {
 if (!uri) return FALSE;
 GError *error=NULL; GUri *u=g_uri_parse(uri,G_URI_FLAGS_NONE,&error);
 if(!u){if(error)g_error_free(error);return FALSE;}
 const char *scheme=g_uri_get_scheme(u),*host=g_uri_get_host(u);
 gboolean valid=scheme && host && (!strcmp(scheme,"https")||!strcmp(scheme,"http")) && g_ascii_strcasecmp(host,"wails.localhost") && !g_uri_get_userinfo(u);
 g_uri_unref(u);return valid;
}
static void changed(WebKitWebView *v, GParamSpec *p, gpointer h) {
 const char *uri=webkit_web_view_get_uri(v),*title=webkit_web_view_get_title(v);
 if(uri)mayakBrowserEvent((uintptr_t)h,(char*)uri,(char*)(title?title:""),webkit_web_view_can_go_back(v),webkit_web_view_can_go_forward(v),0);
}
static gboolean policy(WebKitWebView *v,WebKitPolicyDecision *d,WebKitPolicyDecisionType type,gpointer h) {
 if(type==WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION||type==WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION){
  WebKitNavigationAction *a=webkit_navigation_policy_decision_get_navigation_action(WEBKIT_NAVIGATION_POLICY_DECISION(d));
  const char *uri=webkit_uri_request_get_uri(webkit_navigation_action_get_request(a));
  if(type==WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION){
   webkit_policy_decision_ignore(d);
   if(valid_uri(uri)&&webkit_navigation_action_is_user_gesture(a))mayakBrowserEvent((uintptr_t)h,(char*)uri,"",0,0,1);
   return TRUE;
  }
  if(!valid_uri(uri)){webkit_policy_decision_ignore(d);return TRUE;}
 }
 if(type==WEBKIT_POLICY_DECISION_TYPE_RESPONSE&&!webkit_response_policy_decision_is_mime_type_supported(WEBKIT_RESPONSE_POLICY_DECISION(d))){webkit_policy_decision_ignore(d);return TRUE;}
 return FALSE;
}
static gboolean deny_permission(WebKitWebView *v,WebKitPermissionRequest *r,gpointer h){webkit_permission_request_deny(r);return TRUE;}
void *rl_browser_new(GtkWidget *overlay,uintptr_t handle,const char *directory){
#ifdef MAYAK_GTK3
 WebKitWebsiteDataManager *data=webkit_website_data_manager_new("base-data-directory",directory,NULL);
 WebKitWebContext *context=webkit_web_context_new_with_website_data_manager(data);
 GtkWidget *view=webkit_web_view_new_with_context(context);
 g_object_unref(data);g_object_unref(context);
 gtk_widget_set_no_show_all(view,TRUE);
#else
 WebKitNetworkSession *session=webkit_network_session_new(directory,NULL);
 GtkWidget *view=g_object_new(WEBKIT_TYPE_WEB_VIEW,"network-session",session,NULL);
 g_object_unref(session);gtk_widget_set_visible(view,FALSE);
#endif
 gtk_widget_set_hexpand(view,TRUE);gtk_widget_set_vexpand(view,TRUE);
 gtk_widget_set_halign(view,GTK_ALIGN_FILL);gtk_widget_set_valign(view,GTK_ALIGN_FILL);
 gtk_overlay_add_overlay(GTK_OVERLAY(overlay),view);
 g_signal_connect(view,"notify::uri",G_CALLBACK(changed),(gpointer)handle);
 g_signal_connect(view,"notify::title",G_CALLBACK(changed),(gpointer)handle);
 g_signal_connect(view,"notify::estimated-load-progress",G_CALLBACK(changed),(gpointer)handle);
 g_signal_connect(view,"decide-policy",G_CALLBACK(policy),(gpointer)handle);
 g_signal_connect(view,"permission-request",G_CALLBACK(deny_permission),(gpointer)handle);
 return view;
}
void rl_browser_action(void *ptr,const char *command,const char *url,int left,int top,int right,int bottom){
 WebKitWebView *v=WEBKIT_WEB_VIEW(ptr);GtkWidget *widget=GTK_WIDGET(ptr);
 if(!strcmp(command,"show")){gtk_widget_set_margin_start(widget,left);gtk_widget_set_margin_top(widget,top);gtk_widget_set_margin_end(widget,right);gtk_widget_set_margin_bottom(widget,bottom);gtk_widget_show(widget);}
 else if(!strcmp(command,"hide"))gtk_widget_hide(widget);
 else if(!strcmp(command,"navigate")&&valid_uri(url))webkit_web_view_load_uri(v,url);
 else if(!strcmp(command,"back"))webkit_web_view_go_back(v);
 else if(!strcmp(command,"forward"))webkit_web_view_go_forward(v);
 else if(!strcmp(command,"reload"))webkit_web_view_reload(v);
 else if(!strcmp(command,"close")){webkit_web_view_stop_loading(v);
#ifdef MAYAK_GTK3
 gtk_widget_destroy(widget);
#else
 GtkWidget *parent=gtk_widget_get_parent(widget);
 if(parent)gtk_overlay_remove_overlay(GTK_OVERLAY(parent),widget);
#endif
}
}

void *rl_browser_overlay(void *window){
 GtkWidget *parent=GTK_WIDGET(window);
#ifdef MAYAK_GTK3
 GtkWidget *child=gtk_bin_get_child(GTK_BIN(parent));
#else
 GtkWidget *child=gtk_window_get_child(GTK_WINDOW(parent));
#endif
 if(!child)return NULL;
 GtkWidget *overlay=gtk_overlay_new();g_object_ref(child);
#ifdef MAYAK_GTK3
 gtk_container_remove(GTK_CONTAINER(parent),child);
 gtk_container_add(GTK_CONTAINER(overlay),child);
 gtk_container_add(GTK_CONTAINER(parent),overlay);
#else
 gtk_window_set_child(GTK_WINDOW(parent),NULL);
 gtk_overlay_set_child(GTK_OVERLAY(overlay),child);
 gtk_window_set_child(GTK_WINDOW(parent),overlay);
#endif
 g_object_unref(child);gtk_widget_show(overlay);return overlay;
}
