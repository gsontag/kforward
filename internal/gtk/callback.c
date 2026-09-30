#include "callback.h"

static void callback(gpointer instance, gpointer data) {
  goCallback((uintptr_t)data);
}

static void callback_pointer(gpointer instance, gpointer arg, gpointer data) {
  goCallbackPointer((uintptr_t)data, arg);
}

static gboolean callback_bool(gpointer instance, gboolean arg, gpointer data) {
  return goCallbackBool((uintptr_t)data, arg);
}

static void header_func(GtkListBoxRow *row, GtkListBoxRow *before, gpointer data) {
  goHeaderFunc((uintptr_t)data, row, before);
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

gulong kf_connect_pointer(gpointer instance, const char *signal, uintptr_t handle) {
  return g_signal_connect_data(instance, signal, G_CALLBACK(callback_pointer),
    (gpointer)handle, closure_notify, 0);
}

gulong kf_connect_bool(gpointer instance, const char *signal, uintptr_t handle) {
  return g_signal_connect_data(instance, signal, G_CALLBACK(callback_bool),
    (gpointer)handle, closure_notify, 0);
}

void kf_list_box_set_header_func(GtkListBox *box, uintptr_t handle) {
  gtk_list_box_set_header_func(box, header_func, (gpointer)handle, destroy);
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

void kf_emit_pointer(gpointer instance, const char *signal, gpointer arg) {
  g_signal_emit_by_name(instance, signal, arg);
}
