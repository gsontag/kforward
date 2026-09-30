package gtk

// #include <gtk/gtk.h>
// #include "callback.h"
import "C"

import "unsafe"

// SelectionMode is how the rows of a ListBox can be selected.
type SelectionMode int

// Selection modes.
const (
	SelectionNone   SelectionMode = C.GTK_SELECTION_NONE
	SelectionSingle SelectionMode = C.GTK_SELECTION_SINGLE
)

// ListBox shows its children as rows, in a vertical list.
type ListBox struct {
	Widget
}

// NewListBox returns an empty list.
func NewListBox() *ListBox {
	return &ListBox{newWidget(C.gtk_list_box_new())}
}

func (l *ListBox) box(f func(p *C.GtkListBox)) {
	l.with(func(p *C.GtkWidget) { f((*C.GtkListBox)(unsafe.Pointer(p))) })
}

// Append adds child as the last row. A child other than a ListBoxRow gets
// one of its own.
func (l *ListBox) Append(child Widgetter) {
	l.box(func(p *C.GtkListBox) {
		child.base().with(func(c *C.GtkWidget) { C.gtk_list_box_append(p, c) })
	})
}

// RemoveAll removes every row.
func (l *ListBox) RemoveAll() {
	l.box(func(p *C.GtkListBox) { C.gtk_list_box_remove_all(p) })
}

// RowAtIndex returns the row at index, nil if there is none.
func (l *ListBox) RowAtIndex(index int) *ListBoxRow {
	var row *ListBoxRow
	l.box(func(p *C.GtkListBox) { row = wrapRow(C.gtk_list_box_get_row_at_index(p, C.int(index))) })
	return row
}

// SetSelectionMode sets how the rows can be selected.
func (l *ListBox) SetSelectionMode(mode SelectionMode) {
	l.box(func(p *C.GtkListBox) { C.gtk_list_box_set_selection_mode(p, C.GtkSelectionMode(mode)) })
}

// SetPlaceholder sets the widget shown when the list is empty.
func (l *ListBox) SetPlaceholder(placeholder Widgetter) {
	l.box(func(p *C.GtkListBox) {
		placeholder.base().with(func(c *C.GtkWidget) { C.gtk_list_box_set_placeholder(p, c) })
	})
}

// SetHeaderFunc sets the function giving each row its header, from the row
// and the one before it, nil for the first row. GTK calls it when a row is
// added, and after InvalidateHeaders.
func (l *ListBox) SetHeaderFunc(f func(row, before *ListBoxRow)) {
	l.box(func(p *C.GtkListBox) { C.kf_list_box_set_header_func(p, newHandle(f)) })
}

// InvalidateHeaders makes GTK compute the headers again.
func (l *ListBox) InvalidateHeaders() {
	l.box(func(p *C.GtkListBox) { C.gtk_list_box_invalidate_headers(p) })
}

// ConnectRowActivated calls f with the row the user activates, by a click
// or the keyboard.
func (l *ListBox) ConnectRowActivated(f func(row *ListBoxRow)) {
	l.with(func(p *C.GtkWidget) {
		connectPointer(C.gpointer(p), "row-activated", func(row unsafe.Pointer) {
			f(wrapRow((*C.GtkListBoxRow)(row)))
		})
	})
}

// ListBoxRow is a row of a ListBox.
type ListBoxRow struct {
	Widget
}

// wrapRow returns a ListBoxRow for the row p, nil for a nil p.
func wrapRow(p *C.GtkListBoxRow) *ListBoxRow {
	if p == nil {
		return nil
	}
	return &ListBoxRow{newWidget((*C.GtkWidget)(unsafe.Pointer(p)))}
}

func (r *ListBoxRow) row(f func(p *C.GtkListBoxRow)) {
	r.with(func(p *C.GtkWidget) { f((*C.GtkListBoxRow)(unsafe.Pointer(p))) })
}

// Index returns the position of the row in its list, -1 if it has none.
func (r *ListBoxRow) Index() int {
	index := -1
	r.row(func(p *C.GtkListBoxRow) { index = int(C.gtk_list_box_row_get_index(p)) })
	return index
}

// SetHeader sets the widget shown above the row; nil removes it.
func (r *ListBoxRow) SetHeader(header Widgetter) {
	r.row(func(p *C.GtkListBoxRow) {
		if header == nil {
			C.gtk_list_box_row_set_header(p, nil)
			return
		}
		header.base().with(func(h *C.GtkWidget) { C.gtk_list_box_row_set_header(p, h) })
	})
}

// HasHeader tells whether the row shows a header.
func (r *ListBoxRow) HasHeader() bool {
	has := false
	r.row(func(p *C.GtkListBoxRow) { has = C.gtk_list_box_row_get_header(p) != nil })
	return has
}
