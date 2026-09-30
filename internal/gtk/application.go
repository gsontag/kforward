package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import (
	"errors"
	"unsafe"
)

// Application is a GtkApplication: the single instance of the program on the
// session bus, which owns the windows and runs the main loop.
type Application struct {
	p *C.GtkApplication
}

// NewApplication returns the application named id, such as fr.gsontag.kforward.
func NewApplication(id string) *Application {
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	return &Application{p: C.gtk_application_new(cid, C.G_APPLICATION_DEFAULT_FLAGS)}
}

func (a *Application) gapp() *C.GApplication {
	return (*C.GApplication)(unsafe.Pointer(a.p))
}

// Register registers the application on the session bus, which tells whether
// another instance runs already: see IsRemote.
func (a *Application) Register() error {
	var gerr *C.GError
	if C.g_application_register(a.gapp(), nil, &gerr) == C.FALSE {
		defer C.g_error_free(gerr)
		return errors.New(C.GoString(gerr.message))
	}
	return nil
}

// IsRemote tells whether another instance is the primary one: Run then only
// activates it.
func (a *Application) IsRemote() bool {
	return C.g_application_get_is_remote(a.gapp()) != C.FALSE
}

// Hold keeps the application running without any window.
func (a *Application) Hold() {
	C.g_application_hold(a.gapp())
}

// Quit stops the main loop: Run returns.
func (a *Application) Quit() {
	C.g_application_quit(a.gapp())
}

// ConnectActivate calls f at each launch, the first and the remote ones.
func (a *Application) ConnectActivate(f func()) {
	connect(C.gpointer(a.p), "activate", f)
}

// ConnectShutdown calls f once the main loop has stopped.
func (a *Application) ConnectShutdown(f func()) {
	connect(C.gpointer(a.p), "shutdown", f)
}

// Run runs the main loop until Quit, and returns the exit status. args are
// the command line, program name first, as GApplication parses it.
func (a *Application) Run(args []string) int {
	argv := make([]*C.char, len(args))
	for i, arg := range args {
		argv[i] = C.CString(arg)
		defer C.free(unsafe.Pointer(argv[i]))
	}
	var first **C.char
	if len(argv) > 0 {
		first = &argv[0]
	}
	return int(C.g_application_run(a.gapp(), C.int(len(argv)), first))
}
