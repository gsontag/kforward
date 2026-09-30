package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"
import "unsafe"

// Label shows a text.
type Label struct {
	Widget
}

// NewLabel returns a label showing text.
func NewLabel(text string) *Label {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	return &Label{newWidget(C.gtk_label_new(ctext))}
}

func (l *Label) label(f func(p *C.GtkLabel)) {
	l.with(func(p *C.GtkWidget) { f((*C.GtkLabel)(unsafe.Pointer(p))) })
}

// SetText replaces the text.
func (l *Label) SetText(text string) {
	l.label(func(p *C.GtkLabel) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.gtk_label_set_text(p, ctext)
	})
}

// Text returns the text shown.
func (l *Label) Text() string {
	var text string
	l.label(func(p *C.GtkLabel) { text = C.GoString(C.gtk_label_get_text(p)) })
	return text
}

// SetWrap lets a long text continue on several lines.
func (l *Label) SetWrap(wrap bool) {
	l.label(func(p *C.GtkLabel) { C.gtk_label_set_wrap(p, gbool(wrap)) })
}

// SetXAlign places the text in the label: 0 on the left, 1 on the right.
func (l *Label) SetXAlign(x float32) {
	l.label(func(p *C.GtkLabel) { C.gtk_label_set_xalign(p, C.float(x)) })
}
