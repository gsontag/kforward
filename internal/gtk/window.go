package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import "unsafe"

// Window is a toplevel window. GTK owns it until it is closed: unless it
// hides on close, closing destroys it, and its children with it.
type Window struct {
	Widget
}

// NewWindow returns a window, hidden.
func NewWindow() *Window {
	return &Window{newWidget(C.gtk_window_new())}
}

func (w *Window) window(f func(p *C.GtkWindow)) {
	w.with(func(p *C.GtkWidget) { f((*C.GtkWindow)(unsafe.Pointer(p))) })
}

// SetTitle sets the title of the window.
func (w *Window) SetTitle(title string) {
	w.window(func(p *C.GtkWindow) {
		ctitle := C.CString(title)
		defer C.free(unsafe.Pointer(ctitle))
		C.gtk_window_set_title(p, ctitle)
	})
}

// Title returns the title of the window.
func (w *Window) Title() string {
	var title string
	w.window(func(p *C.GtkWindow) { title = C.GoString(C.gtk_window_get_title(p)) })
	return title
}

// SetDefaultSize sets the size of the window when first shown, in pixels;
// -1 lets GTK choose.
func (w *Window) SetDefaultSize(width, height int) {
	w.window(func(p *C.GtkWindow) { C.gtk_window_set_default_size(p, C.int(width), C.int(height)) })
}

// SetHideOnClose makes closing the window only hide it.
func (w *Window) SetHideOnClose(hide bool) {
	w.window(func(p *C.GtkWindow) { C.gtk_window_set_hide_on_close(p, gbool(hide)) })
}

// SetTitlebar replaces the title bar with a widget, usually a HeaderBar.
func (w *Window) SetTitlebar(titlebar Widgetter) {
	w.window(func(p *C.GtkWindow) {
		titlebar.base().with(func(t *C.GtkWidget) { C.gtk_window_set_titlebar(p, t) })
	})
}

// SetChild sets the content of the window.
func (w *Window) SetChild(child Widgetter) {
	w.window(func(p *C.GtkWindow) {
		child.base().with(func(c *C.GtkWidget) { C.gtk_window_set_child(p, c) })
	})
}

// SetTransientFor keeps the window above parent, as a dialog of it.
func (w *Window) SetTransientFor(parent *Window) {
	w.window(func(p *C.GtkWindow) {
		parent.window(func(pp *C.GtkWindow) { C.gtk_window_set_transient_for(p, pp) })
	})
}

// SetModal makes the window block its parent while shown.
func (w *Window) SetModal(modal bool) {
	w.window(func(p *C.GtkWindow) { C.gtk_window_set_modal(p, gbool(modal)) })
}

// SetDestroyWithParent destroys the window with its parent.
func (w *Window) SetDestroyWithParent(destroy bool) {
	w.window(func(p *C.GtkWindow) { C.gtk_window_set_destroy_with_parent(p, gbool(destroy)) })
}

// SetDefaultWidget sets the widget Enter activates, in an entry that
// activates the default.
func (w *Window) SetDefaultWidget(widget Widgetter) {
	w.window(func(p *C.GtkWindow) {
		widget.base().with(func(d *C.GtkWidget) { C.gtk_window_set_default_widget(p, d) })
	})
}

// Present shows the window, in front of the others.
func (w *Window) Present() {
	w.window(func(p *C.GtkWindow) { C.gtk_window_present(p) })
}

// Close closes the window, as its close button does.
func (w *Window) Close() {
	w.window(func(p *C.GtkWindow) { C.gtk_window_close(p) })
}

// ApplicationWindow is a window of the application: the application runs as
// long as it has one, and a second launch can bring it to the front.
type ApplicationWindow struct {
	Window
}

// NewApplicationWindow returns a window of app, hidden.
func NewApplicationWindow(app *Application) *ApplicationWindow {
	return &ApplicationWindow{Window{newWidget(C.gtk_application_window_new(app.p))}}
}

// HeaderBar is a title bar holding widgets, beside the title.
type HeaderBar struct {
	Widget
}

// NewHeaderBar returns a header bar with the title of its window.
func NewHeaderBar() *HeaderBar {
	return &HeaderBar{newWidget(C.gtk_header_bar_new())}
}

func (h *HeaderBar) bar(f func(p *C.GtkHeaderBar)) {
	h.with(func(p *C.GtkWidget) { f((*C.GtkHeaderBar)(unsafe.Pointer(p))) })
}

// PackStart adds child at the start of the bar, on the left in left-to-right
// languages.
func (h *HeaderBar) PackStart(child Widgetter) {
	h.bar(func(p *C.GtkHeaderBar) {
		child.base().with(func(c *C.GtkWidget) { C.gtk_header_bar_pack_start(p, c) })
	})
}

// PackEnd adds child at the end of the bar.
func (h *HeaderBar) PackEnd(child Widgetter) {
	h.bar(func(p *C.GtkHeaderBar) {
		child.base().with(func(c *C.GtkWidget) { C.gtk_header_bar_pack_end(p, c) })
	})
}

// SetShowTitleButtons shows or hides the buttons that close, minimize...
// the window.
func (h *HeaderBar) SetShowTitleButtons(show bool) {
	h.bar(func(p *C.GtkHeaderBar) { C.gtk_header_bar_set_show_title_buttons(p, gbool(show)) })
}
