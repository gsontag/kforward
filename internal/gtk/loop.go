package gtk

// #include "callback.h"
import "C"
import "runtime/cgo"

// SourceHandle identifies a function scheduled on the main loop.
type SourceHandle uint

// IdleAdd runs f once on the main loop, as soon as it is idle. It is safe to
// call from any goroutine: it is how other goroutines reach the UI.
func IdleAdd(f func()) {
	C.kf_idle_add(C.uintptr_t(cgo.NewHandle(once(f))))
}

// TimeoutAdd runs f once on the main loop, after ms milliseconds, unless the
// source is removed first.
func TimeoutAdd(ms uint, f func()) SourceHandle {
	return SourceHandle(C.kf_timeout_add(C.guint(ms), C.uintptr_t(cgo.NewHandle(once(f)))))
}

// once adapts f to a GSourceFunc returning false: GLib then removes the
// source, and releases the handle.
func once(f func()) func() bool {
	return func() bool {
		f()
		return false
	}
}

// SourceRemove cancels a function scheduled on the main loop.
func SourceRemove(h SourceHandle) {
	C.g_source_remove(C.guint(h))
}

// iterate runs what is pending on the main loop, without waiting: the tests
// drive the loop without an application.
func iterate() {
	for C.g_main_context_pending(nil) != 0 {
		C.g_main_context_iteration(nil, C.FALSE)
	}
}
