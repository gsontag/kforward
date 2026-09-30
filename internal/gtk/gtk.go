// Package gtk binds the few GTK and GLib functions kforward uses. Binding
// only those, rather than the whole API, keeps the build fast, the binary
// small, and the required versions those of the functions really called.
package gtk

// #cgo pkg-config: gtk4
import "C"
