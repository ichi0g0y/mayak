#include <stdint.h>
void *rl_browser_new(void *context, uintptr_t handle);
void rl_browser_action(void *view, const char *command, const char *url, int left, int top, int right, int bottom);
