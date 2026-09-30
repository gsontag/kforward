package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import "unsafe"

// Button runs an action when clicked.
type Button struct {
	Widget
}

// NewButtonWithLabel returns a button showing text.
func NewButtonWithLabel(text string) *Button {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	return &Button{newWidget(C.gtk_button_new_with_label(ctext))}
}

// NewButtonFromIconName returns a button showing the icon of the theme
// named icon, such as "list-add-symbolic".
func NewButtonFromIconName(icon string) *Button {
	cicon := C.CString(icon)
	defer C.free(unsafe.Pointer(cicon))
	return &Button{newWidget(C.gtk_button_new_from_icon_name(cicon))}
}

// SetLabel replaces the text of the button.
func (b *Button) SetLabel(text string) {
	b.with(func(p *C.GtkWidget) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.gtk_button_set_label((*C.GtkButton)(unsafe.Pointer(p)), ctext)
	})
}

// Label returns the text of the button.
func (b *Button) Label() string {
	var text string
	b.with(func(p *C.GtkWidget) {
		text = C.GoString(C.gtk_button_get_label((*C.GtkButton)(unsafe.Pointer(p))))
	})
	return text
}

// ConnectClicked calls f at each click.
func (b *Button) ConnectClicked(f func()) {
	b.with(func(p *C.GtkWidget) { connect(C.gpointer(p), "clicked", f) })
}
