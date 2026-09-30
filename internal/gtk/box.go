package gtk

// #include <gtk/gtk.h>
import "C"
import "unsafe"

// Box lays its children out in a row or a column.
type Box struct {
	Widget
}

// NewBox returns an empty box, with spacing pixels between its children.
func NewBox(o Orientation, spacing int) *Box {
	return &Box{newWidget(C.gtk_box_new(C.GtkOrientation(o), C.int(spacing)))}
}

// Append adds child after the other children.
func (b *Box) Append(child Widgetter) {
	b.with(func(p *C.GtkWidget) {
		child.base().with(func(c *C.GtkWidget) {
			C.gtk_box_append((*C.GtkBox)(unsafe.Pointer(p)), c)
		})
	})
}
