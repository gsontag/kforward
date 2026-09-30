package gtk

// #include <gtk/gtk.h>
import "C"

import "unsafe"

// Grid lays its children out in rows and columns.
type Grid struct {
	Widget
}

// NewGrid returns an empty grid.
func NewGrid() *Grid {
	return &Grid{newWidget(C.gtk_grid_new())}
}

func (g *Grid) grid(f func(p *C.GtkGrid)) {
	g.with(func(p *C.GtkWidget) { f((*C.GtkGrid)(unsafe.Pointer(p))) })
}

// Attach places child at column, row, over width columns and height rows.
func (g *Grid) Attach(child Widgetter, column, row, width, height int) {
	g.grid(func(p *C.GtkGrid) {
		child.base().with(func(c *C.GtkWidget) {
			C.gtk_grid_attach(p, c, C.int(column), C.int(row), C.int(width), C.int(height))
		})
	})
}

// SetRowSpacing sets the space between rows, in pixels.
func (g *Grid) SetRowSpacing(spacing uint) {
	g.grid(func(p *C.GtkGrid) { C.gtk_grid_set_row_spacing(p, C.guint(spacing)) })
}

// SetColumnSpacing sets the space between columns, in pixels.
func (g *Grid) SetColumnSpacing(spacing uint) {
	g.grid(func(p *C.GtkGrid) { C.gtk_grid_set_column_spacing(p, C.guint(spacing)) })
}
