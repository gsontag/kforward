package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"
import "unsafe"

// Align is how a widget uses the space it gets beyond its own size.
type Align int

// Alignments.
const (
	AlignFill   Align = C.GTK_ALIGN_FILL
	AlignStart  Align = C.GTK_ALIGN_START
	AlignEnd    Align = C.GTK_ALIGN_END
	AlignCenter Align = C.GTK_ALIGN_CENTER
)

// Orientation is the direction a Box lays its children out in.
type Orientation int

// Orientations.
const (
	OrientationHorizontal Orientation = C.GTK_ORIENTATION_HORIZONTAL
	OrientationVertical   Orientation = C.GTK_ORIENTATION_VERTICAL
)

// Widget is what every widget has in common.
type Widget struct {
	obj *object
}

// Widgetter is any widget: a *Box, a *Label... as well as a *Widget.
type Widgetter interface {
	base() *Widget
}

func (w *Widget) base() *Widget { return w }

func newWidget(p *C.GtkWidget) Widget {
	return Widget{obj: newObject(unsafe.Pointer(p))}
}

// with calls f with the widget, unless it is gone.
func (w *Widget) with(f func(p *C.GtkWidget)) {
	w.obj.with(func(p unsafe.Pointer) { f((*C.GtkWidget)(p)) })
}

// SetMarginTop sets the space above the widget, in pixels.
func (w *Widget) SetMarginTop(margin int) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_margin_top(p, C.int(margin)) })
}

// SetMarginBottom sets the space below the widget, in pixels.
func (w *Widget) SetMarginBottom(margin int) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_margin_bottom(p, C.int(margin)) })
}

// SetMarginStart sets the space before the widget, in pixels: on its left in
// left-to-right languages.
func (w *Widget) SetMarginStart(margin int) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_margin_start(p, C.int(margin)) })
}

// SetMarginEnd sets the space after the widget, in pixels: on its right in
// left-to-right languages.
func (w *Widget) SetMarginEnd(margin int) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_margin_end(p, C.int(margin)) })
}

// SetHAlign sets the horizontal alignment.
func (w *Widget) SetHAlign(a Align) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_halign(p, C.GtkAlign(a)) })
}

// SetVAlign sets the vertical alignment.
func (w *Widget) SetVAlign(a Align) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_valign(p, C.GtkAlign(a)) })
}

// SetHExpand makes the widget take the horizontal space left.
func (w *Widget) SetHExpand(expand bool) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_hexpand(p, gbool(expand)) })
}

// SetVExpand makes the widget take the vertical space left.
func (w *Widget) SetVExpand(expand bool) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_vexpand(p, gbool(expand)) })
}

// SetVisible shows or hides the widget.
func (w *Widget) SetVisible(visible bool) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_visible(p, gbool(visible)) })
}

// IsVisible tells whether the widget and its parents are shown: false once
// the widget is gone.
func (w *Widget) IsVisible() bool {
	visible := false
	w.with(func(p *C.GtkWidget) { visible = C.gtk_widget_is_visible(p) != C.FALSE })
	return visible
}

// SetSensitive enables or disables the widget.
func (w *Widget) SetSensitive(sensitive bool) {
	w.with(func(p *C.GtkWidget) { C.gtk_widget_set_sensitive(p, gbool(sensitive)) })
}

// SetTooltipText sets the text shown when the pointer rests on the widget.
func (w *Widget) SetTooltipText(text string) {
	w.with(func(p *C.GtkWidget) {
		ctext := C.CString(text)
		defer C.free(unsafe.Pointer(ctext))
		C.gtk_widget_set_tooltip_text(p, ctext)
	})
}

// AddCSSClass adds a style class, such as "error" or "dim-label".
func (w *Widget) AddCSSClass(class string) {
	w.with(func(p *C.GtkWidget) {
		cclass := C.CString(class)
		defer C.free(unsafe.Pointer(cclass))
		C.gtk_widget_add_css_class(p, cclass)
	})
}

// RemoveCSSClass removes a style class.
func (w *Widget) RemoveCSSClass(class string) {
	w.with(func(p *C.GtkWidget) {
		cclass := C.CString(class)
		defer C.free(unsafe.Pointer(cclass))
		C.gtk_widget_remove_css_class(p, cclass)
	})
}

// NotifyProperty calls f each time the property changes, such as "visible".
func (w *Widget) NotifyProperty(property string, f func()) {
	w.with(func(p *C.GtkWidget) {
		connectPointer(C.gpointer(p), "notify::"+property, func(unsafe.Pointer) { f() })
	})
}

// Native is the GtkWidget pointer, for the code still using gotk4; nil
// once the widget is gone.
func (w *Widget) Native() unsafe.Pointer {
	var native unsafe.Pointer
	w.with(func(p *C.GtkWidget) { native = unsafe.Pointer(p) })
	return native
}

// ConnectDestroy calls f when the widget is destroyed.
func (w *Widget) ConnectDestroy(f func()) {
	w.with(func(p *C.GtkWidget) { connect(C.gpointer(p), "destroy", f) })
}

func gbool(b bool) C.gboolean {
	if b {
		return C.TRUE
	}
	return C.FALSE
}
