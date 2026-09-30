package gtk

// #include "callback.h"
// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import (
	"runtime"
	"unsafe"
)

// object designates a GObject without owning it. GTK owns the widgets: a
// parent its children, GTK the windows. Owning them from Go would make
// cycles, a signal handler keeping alive the Go value that keeps its widget
// alive, and neither would ever be freed. A weak reference instead tells
// when the object is gone: the methods then do nothing, rather than touching
// freed memory, as a late callback of the main loop could.
type object struct {
	// ref lives in C memory: GLib updates it when the object is finalized
	ref *C.GWeakRef
}

func newObject(p unsafe.Pointer) *object {
	o := &object{ref: (*C.GWeakRef)(C.calloc(1, C.sizeof_GWeakRef))}
	C.g_weak_ref_init(o.ref, C.gpointer(p))
	// Weak references are thread-safe: no need to reach the main loop
	runtime.AddCleanup(o, func(ref *C.GWeakRef) {
		C.g_weak_ref_clear(ref)
		C.free(unsafe.Pointer(ref))
	}, o.ref)
	return o
}

// with calls f with the object, held for the call, unless it is gone.
func (o *object) with(f func(p unsafe.Pointer)) {
	p := C.g_weak_ref_get(o.ref)
	if p == nil {
		return
	}
	defer C.g_object_unref(p)
	f(unsafe.Pointer(p))
}

// alive tells whether the object still exists.
func (o *object) alive() bool {
	alive := false
	o.with(func(unsafe.Pointer) { alive = true })
	return alive
}

// emit emits signal on the object, without arguments: the tests check the
// callbacks without a user to click.
func (o *object) emit(signal string) {
	o.with(func(p unsafe.Pointer) { emit(p, signal) })
}

// emitWith emits signal on the object, with arg as its only argument.
func (o *object) emitWith(signal string, arg *object) {
	o.with(func(p unsafe.Pointer) {
		arg.with(func(a unsafe.Pointer) {
			name := C.CString(signal)
			defer C.free(unsafe.Pointer(name))
			C.kf_emit_pointer(C.gpointer(p), name, C.gpointer(a))
		})
	})
}

// release drops the reference of a widget never added to a parent: the tests
// free what they created.
func (o *object) release() {
	o.with(func(p unsafe.Pointer) {
		C.g_object_ref_sink(C.gpointer(p))
		C.g_object_unref(C.gpointer(p))
	})
}
