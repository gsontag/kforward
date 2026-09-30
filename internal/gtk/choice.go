package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
import "C"

import "unsafe"

// DropDown lets the user choose one item of a list.
type DropDown struct {
	Widget
}

// NewDropDownFromStrings returns a drop-down of items, the first selected.
func NewDropDownFromStrings(items []string) *DropDown {
	// A NULL-terminated array of C strings, freed once GTK has copied them
	cstrs := make([]*C.char, len(items)+1)
	for i, item := range items {
		cstrs[i] = C.CString(item)
		defer C.free(unsafe.Pointer(cstrs[i]))
	}
	return &DropDown{newWidget(C.gtk_drop_down_new_from_strings(&cstrs[0]))}
}

func (d *DropDown) dropDown(f func(p *C.GtkDropDown)) {
	d.with(func(p *C.GtkWidget) { f((*C.GtkDropDown)(unsafe.Pointer(p))) })
}

// Selected returns the index of the selected item.
func (d *DropDown) Selected() uint {
	var selected uint
	d.dropDown(func(p *C.GtkDropDown) { selected = uint(C.gtk_drop_down_get_selected(p)) })
	return selected
}

// SetSelected selects the item at index. Watch the "selected" property to
// learn of the changes, the user's and these.
func (d *DropDown) SetSelected(index uint) {
	d.dropDown(func(p *C.GtkDropDown) { C.gtk_drop_down_set_selected(p, C.guint(index)) })
}

// CheckButton is a check box with its label.
type CheckButton struct {
	Widget
}

// NewCheckButtonWithLabel returns a check box, unchecked, labelled text.
func NewCheckButtonWithLabel(text string) *CheckButton {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	return &CheckButton{newWidget(C.gtk_check_button_new_with_label(ctext))}
}

func (c *CheckButton) check(f func(p *C.GtkCheckButton)) {
	c.with(func(p *C.GtkWidget) { f((*C.GtkCheckButton)(unsafe.Pointer(p))) })
}

// Active tells whether the box is checked.
func (c *CheckButton) Active() bool {
	active := false
	c.check(func(p *C.GtkCheckButton) { active = C.gtk_check_button_get_active(p) != C.FALSE })
	return active
}

// SetActive checks or unchecks the box.
func (c *CheckButton) SetActive(active bool) {
	c.check(func(p *C.GtkCheckButton) { C.gtk_check_button_set_active(p, gbool(active)) })
}
