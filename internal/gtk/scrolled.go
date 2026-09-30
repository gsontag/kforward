package gtk

// #include <gtk/gtk.h>
import "C"

import "unsafe"

// ScrolledWindow shows a part of its child, with scrollbars.
type ScrolledWindow struct {
	Widget
}

// NewScrolledWindow returns a scrolled window without child.
func NewScrolledWindow() *ScrolledWindow {
	return &ScrolledWindow{newWidget(C.gtk_scrolled_window_new())}
}

func (s *ScrolledWindow) scrolled(f func(p *C.GtkScrolledWindow)) {
	s.with(func(p *C.GtkWidget) { f((*C.GtkScrolledWindow)(unsafe.Pointer(p))) })
}

// SetChild sets the widget to scroll.
func (s *ScrolledWindow) SetChild(child Widgetter) {
	s.scrolled(func(p *C.GtkScrolledWindow) {
		child.base().with(func(c *C.GtkWidget) { C.gtk_scrolled_window_set_child(p, c) })
	})
}

// SetMaxContentHeight limits the height the window asks for, in pixels.
func (s *ScrolledWindow) SetMaxContentHeight(height int) {
	s.scrolled(func(p *C.GtkScrolledWindow) {
		C.gtk_scrolled_window_set_max_content_height(p, C.int(height))
	})
}

// SetPropagateNaturalWidth makes the window as wide as its child wants to be.
func (s *ScrolledWindow) SetPropagateNaturalWidth(propagate bool) {
	s.scrolled(func(p *C.GtkScrolledWindow) {
		C.gtk_scrolled_window_set_propagate_natural_width(p, gbool(propagate))
	})
}

// SetPropagateNaturalHeight makes the window as high as its child wants to
// be, up to the maximum content height.
func (s *ScrolledWindow) SetPropagateNaturalHeight(propagate bool) {
	s.scrolled(func(p *C.GtkScrolledWindow) {
		C.gtk_scrolled_window_set_propagate_natural_height(p, gbool(propagate))
	})
}
