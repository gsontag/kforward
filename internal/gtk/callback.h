#include <gtk/gtk.h>

// The Go side, exported by callback.go. Declared here rather than taken from
// _cgo_export.h, which cgo generates during the build only: editors cannot
// find it
extern void goCallback(uintptr_t handle);
extern gboolean goSourceFunc(uintptr_t handle);
extern void goRelease(uintptr_t handle);

// The trampolines: C functions GLib can call, which call back into Go with the
// cgo.Handle of the Go function, carried as the user data of the callback.
gulong kf_connect(gpointer instance, const char *signal, uintptr_t handle);
guint kf_idle_add(uintptr_t handle);
void kf_emit(gpointer instance, const char *signal);
guint kf_timeout_add(guint ms, uintptr_t handle);
