#include "callback.h"

static void callback(gpointer instance, gpointer data) {
  goCallback((uintptr_t)data);
}

static gboolean source_func(gpointer data) {
  return goSourceFunc((uintptr_t)data);
}

static void destroy(gpointer data) {
  goRelease((uintptr_t)data);
}

static void closure_notify(gpointer data, GClosure *closure) {
  goRelease((uintptr_t)data);
}

gulong kf_connect(gpointer instance, const char *signal, uintptr_t handle) {
  return g_signal_connect_data(instance, signal, G_CALLBACK(callback),
    (gpointer)handle, closure_notify, 0);
}

guint kf_idle_add(uintptr_t handle) {
  return g_idle_add_full(G_PRIORITY_DEFAULT_IDLE, source_func, (gpointer)handle, destroy);
}

guint kf_timeout_add(guint ms, uintptr_t handle) {
  return g_timeout_add_full(G_PRIORITY_DEFAULT, ms, source_func, (gpointer)handle, destroy);
}

void kf_emit(gpointer instance, const char *signal) {
  g_signal_emit_by_name(instance, signal);
}
