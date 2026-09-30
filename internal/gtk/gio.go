package gtk

// #include <stdlib.h>
// #include <gtk/gtk.h>
// #include "callback.h"
import "C"

import (
	"errors"
	"runtime"
	"unsafe"
)

// FileURI returns the file:// URI of the file at path, escaped as needed.
func FileURI(path string) string {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	file := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(file))
	uri := C.g_file_get_uri(file)
	// Unlike the strings of the widgets, this one is ours to free
	defer C.g_free(C.gpointer(uri))
	return C.GoString(uri)
}

// LaunchDefaultForURI opens uri with the application the desktop chose for
// it: the browser for an http URI, an editor for a text file...
func LaunchDefaultForURI(uri string) error {
	curi := C.CString(uri)
	defer C.free(unsafe.Pointer(curi))
	var gerr *C.GError
	if C.g_app_info_launch_default_for_uri(curi, nil, &gerr) == C.FALSE {
		return takeError(gerr)
	}
	return nil
}

// takeError converts and frees a GError.
func takeError(gerr *C.GError) error {
	defer C.g_error_free(gerr)
	return errors.New(C.GoString(gerr.message))
}

// FileMonitorEvent is what happened to a monitored file.
type FileMonitorEvent int

// File monitor events.
const (
	FileMonitorEventChanged          FileMonitorEvent = C.G_FILE_MONITOR_EVENT_CHANGED
	FileMonitorEventChangesDoneHint  FileMonitorEvent = C.G_FILE_MONITOR_EVENT_CHANGES_DONE_HINT
	FileMonitorEventDeleted          FileMonitorEvent = C.G_FILE_MONITOR_EVENT_DELETED
	FileMonitorEventCreated          FileMonitorEvent = C.G_FILE_MONITOR_EVENT_CREATED
	FileMonitorEventAttributeChanged FileMonitorEvent = C.G_FILE_MONITOR_EVENT_ATTRIBUTE_CHANGED
	FileMonitorEventPreUnmount       FileMonitorEvent = C.G_FILE_MONITOR_EVENT_PRE_UNMOUNT
	FileMonitorEventUnmounted        FileMonitorEvent = C.G_FILE_MONITOR_EVENT_UNMOUNTED
	FileMonitorEventMoved            FileMonitorEvent = C.G_FILE_MONITOR_EVENT_MOVED
	FileMonitorEventRenamed          FileMonitorEvent = C.G_FILE_MONITOR_EVENT_RENAMED
	FileMonitorEventMovedIn          FileMonitorEvent = C.G_FILE_MONITOR_EVENT_MOVED_IN
	FileMonitorEventMovedOut         FileMonitorEvent = C.G_FILE_MONITOR_EVENT_MOVED_OUT
)

// FileMonitor watches a file. Unlike the widgets, nothing else owns it: the
// Go value does, and the monitor stops once the value is collected.
type FileMonitor struct {
	p *C.GFileMonitor
}

// MonitorFile watches the file at path, which may not exist yet. Moves are
// reported as such: a save by rename is a rename, not a deletion.
func MonitorFile(path string) (*FileMonitor, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	file := C.g_file_new_for_path(cpath)
	defer C.g_object_unref(C.gpointer(file))

	var gerr *C.GError
	p := C.g_file_monitor_file(file, C.G_FILE_MONITOR_WATCH_MOVES, nil, &gerr)
	if p == nil {
		return nil, takeError(gerr)
	}
	m := &FileMonitor{p: p}
	// Released on the main loop, where the monitor delivers its events
	runtime.AddCleanup(m, func(p *C.GFileMonitor) {
		IdleAdd(func() { C.g_object_unref(C.gpointer(p)) })
	}, p)
	return m, nil
}

// ConnectChanged calls f with each event, on the main loop.
func (m *FileMonitor) ConnectChanged(f func(event FileMonitorEvent)) {
	C.kf_connect_file_changed(m.p, newHandle(f))
}
