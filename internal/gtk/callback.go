package gtk

// #include <stdlib.h>
// #include "callback.h"
import "C"

import (
	"runtime/cgo"
	"sync/atomic"
	"unsafe"
)

// A Go function cannot be given to C: it is registered as a cgo.Handle, an
// integer C can hold, and called back through the trampolines of callback.c.
// GLib releases the handle once the callback can no longer run.

//export goCallback
func goCallback(handle C.uintptr_t) {
	cgo.Handle(handle).Value().(func())()
}

//export goSourceFunc
func goSourceFunc(handle C.uintptr_t) C.gboolean {
	if cgo.Handle(handle).Value().(func() bool)() {
		return C.TRUE
	}
	return C.FALSE
}

//export goRelease
func goRelease(handle C.uintptr_t) {
	cgo.Handle(handle).Delete()
	live.Add(-1)
}

// live counts the handles GLib has not released yet: a callback it keeps
// forever is a leak, which the tests look for.
var live atomic.Int64

func newHandle(f any) C.uintptr_t {
	live.Add(1)
	return C.uintptr_t(cgo.NewHandle(f))
}

// connect calls f each time instance emits signal. The signal must pass no
// argument but the instance, and return nothing.
func connect(instance C.gpointer, signal string, f func()) {
	name := C.CString(signal)
	defer C.free(unsafe.Pointer(name))
	C.kf_connect(instance, name, newHandle(f))
}

// emit emits signal on instance, without arguments: the tests check the
// trampoline without a running application.
func emit(instance unsafe.Pointer, signal string) {
	name := C.CString(signal)
	defer C.free(unsafe.Pointer(name))
	C.kf_emit(C.gpointer(instance), name)
}
