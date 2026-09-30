// Package gtk binds the few GTK and GLib functions kforward uses. Binding
// only those, rather than the whole API, keeps the build fast, the binary
// small, and the required versions those of the functions really called.
package gtk

// #cgo pkg-config: gtk4
// #include <gtk/gtk.h>
import "C"

import "runtime"

// GTK must be used from the thread that initialized it. The main goroutine
// runs on the main thread, and stays on it: main initializes GTK and runs
// its loop, from which every callback comes.
func init() {
	runtime.LockOSThread()
}

// initGTK initializes GTK without an application, for the tests of the
// widgets. It fails without a display.
func initGTK() bool {
	return C.gtk_init_check() != C.FALSE
}
