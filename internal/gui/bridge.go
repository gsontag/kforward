package gui

import (
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	gotk "github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// gotkWindow wraps the window for the editor, which still uses gotk4, until
// the migration to internal/gtk is over.
func (w *Window) gotkWindow() *gotk.Window {
	return &coreglib.Take(w.win.Native()).Cast().(*gotk.ApplicationWindow).Window
}
