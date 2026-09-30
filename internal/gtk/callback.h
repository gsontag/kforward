#include <gtk/gtk.h>

// The Go side, exported by callback.go. Declared here rather than taken from
// _cgo_export.h, which cgo generates during the build only: editors cannot
// find it
extern void goCallback(uintptr_t handle);
extern gboolean goSourceFunc(uintptr_t handle);
extern void goRelease(uintptr_t handle);
extern void goCallbackPointer(uintptr_t handle, gpointer arg);
extern gboolean goCallbackBool(uintptr_t handle, gboolean arg);
extern void goFileChanged(uintptr_t handle, GFileMonitorEvent event);
extern void goHeaderFunc(uintptr_t handle, GtkListBoxRow *row, GtkListBoxRow *before);

// The trampolines: C functions GLib can call, which call back into Go with the
// cgo.Handle of the Go function, carried as the user data of the callback.
gulong kf_connect(gpointer instance, const char *signal, uintptr_t handle);
gulong kf_connect_pointer(gpointer instance, const char *signal, uintptr_t handle);
gulong kf_connect_bool(gpointer instance, const char *signal, uintptr_t handle);
void kf_list_box_set_header_func(GtkListBox *box, uintptr_t handle);
gulong kf_connect_file_changed(GFileMonitor *monitor, uintptr_t handle);
guint kf_idle_add(uintptr_t handle);
void kf_emit(gpointer instance, const char *signal);
void kf_emit_pointer(gpointer instance, const char *signal, gpointer arg);
guint kf_timeout_add(guint ms, uintptr_t handle);
