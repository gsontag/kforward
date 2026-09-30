package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import "unsafe"

// Entry is a one-line text field.
type Entry struct {
	Widget
}

// NewEntry returns an empty entry.
func NewEntry() *Entry {
	return &Entry{newWidget(C.gtk_entry_new())}
}

func (e *Entry) entry(f func(p *C.GtkEntry)) {
	e.with(func(p *C.GtkWidget) { f((*C.GtkEntry)(unsafe.Pointer(p))) })
}

// editable is the GtkEditable interface of the entry, which holds its text.
func (e *Entry) editable(f func(p *C.GtkEditable)) {
	e.with(func(p *C.GtkWidget) { f((*C.GtkEditable)(unsafe.Pointer(p))) })
}

// SetText replaces the text. It emits changed as typing would: twice when
// there was a text, once emptied, then filled.
func (e *Entry) SetText(text string) {
	e.editable(func(p *C.GtkEditable) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.gtk_editable_set_text(p, ctext)
	})
}

// Text returns the text.
func (e *Entry) Text() string {
	var text string
	e.editable(func(p *C.GtkEditable) { text = C.GoString(C.gtk_editable_get_text(p)) })
	return text
}

// SetPlaceholderText sets the text shown, dimmed, while the entry is empty.
func (e *Entry) SetPlaceholderText(text string) {
	e.entry(func(p *C.GtkEntry) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.gtk_entry_set_placeholder_text(p, ctext)
	})
}

// SetActivatesDefault makes Enter activate the default widget of the window.
func (e *Entry) SetActivatesDefault(activates bool) {
	e.entry(func(p *C.GtkEntry) { C.gtk_entry_set_activates_default(p, gbool(activates)) })
}

// ConnectChanged calls f each time the text changes.
func (e *Entry) ConnectChanged(f func()) {
	e.with(func(p *C.GtkWidget) { connect(C.gpointer(p), "changed", f) })
}
