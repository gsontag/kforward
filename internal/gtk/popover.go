package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import "unsafe"

// Popover is a bubble shown next to the widget it belongs to.
type Popover struct {
	Widget
}

// NewPopover returns an empty popover.
func NewPopover() *Popover {
	return &Popover{newWidget(C.gtk_popover_new())}
}

func (p *Popover) popover(f func(p *C.GtkPopover)) {
	p.with(func(w *C.GtkWidget) { f((*C.GtkPopover)(unsafe.Pointer(w))) })
}

// SetChild sets the content of the popover.
func (p *Popover) SetChild(child Widgetter) {
	p.popover(func(pp *C.GtkPopover) {
		child.base().with(func(c *C.GtkWidget) { C.gtk_popover_set_child(pp, c) })
	})
}

// Popdown hides the popover.
func (p *Popover) Popdown() {
	p.popover(func(pp *C.GtkPopover) { C.gtk_popover_popdown(pp) })
}

// MenuButton shows a popover when clicked.
type MenuButton struct {
	Widget
}

// NewMenuButton returns a menu button without popover.
func NewMenuButton() *MenuButton {
	return &MenuButton{newWidget(C.gtk_menu_button_new())}
}

func (m *MenuButton) button(f func(p *C.GtkMenuButton)) {
	m.with(func(p *C.GtkWidget) { f((*C.GtkMenuButton)(unsafe.Pointer(p))) })
}

// SetIconName shows the icon of the theme named icon on the button.
func (m *MenuButton) SetIconName(icon string) {
	m.button(func(p *C.GtkMenuButton) {
		cicon := C.CString(icon)
		defer C.free(unsafe.Pointer(cicon))
		C.gtk_menu_button_set_icon_name(p, cicon)
	})
}

// SetPopover sets the popover the button shows; the button owns it.
func (m *MenuButton) SetPopover(popover *Popover) {
	m.button(func(p *C.GtkMenuButton) {
		popover.with(func(pp *C.GtkWidget) { C.gtk_menu_button_set_popover(p, pp) })
	})
}
