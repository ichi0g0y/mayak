#include <gtk/gtk.h>
#ifdef MAYAK_GTK3
#include <webkit2/webkit2.h>
#else
#include <webkit/webkit.h>
#endif
#include <stdint.h>
void *rl_browser_new(GtkWidget *, uintptr_t, const char *);
void rl_browser_action(void *, const char *, const char *, int, int, int, int);

void *rl_browser_overlay(void *window);
