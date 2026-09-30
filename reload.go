package main

import (
	"log/slog"

	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/gtk"
)

// reloadDelay lets an editor finish writing: several events come for a save.
const reloadDelay = 1000 // milliseconds

// watchConfig reloads the configuration when its file changes, such as when
// edited by hand.
func (a *app) watchConfig() {
	if a.path == "" {
		return
	}

	// Watch moves too: a save by rename, like the store's own, is not a write
	monitor, err := gtk.MonitorFile(a.path)
	if err != nil {
		slog.Warn("watch config", "path", a.path, "err", err)
		return
	}
	// Kept in the app: a monitor collected by the GC stops watching
	a.monitor = monitor
	a.monitor.ConnectChanged(func(event gtk.FileMonitorEvent) {
		switch event {
		case gtk.FileMonitorEventChangesDoneHint,
			gtk.FileMonitorEventCreated,
			gtk.FileMonitorEventDeleted,
			gtk.FileMonitorEventRenamed,
			gtk.FileMonitorEventMovedIn:
			a.scheduleReload()
		// A write in progress, a permission change, a move out, an unmount:
		// nothing to read, or ChangesDoneHint follows
		case gtk.FileMonitorEventChanged,
			gtk.FileMonitorEventAttributeChanged,
			gtk.FileMonitorEventMoved,
			gtk.FileMonitorEventMovedOut,
			gtk.FileMonitorEventPreUnmount,
			gtk.FileMonitorEventUnmounted:
		}
	})
}

// scheduleReload reloads once the events stop: each one postpones it.
func (a *app) scheduleReload() {
	if a.reloadTimer != 0 {
		gtk.SourceRemove(a.reloadTimer)
	}
	a.reloadTimer = gtk.TimeoutAdd(reloadDelay, func() {
		a.reloadTimer = 0
		a.reload()
	})
}

// reload applies the file: the forwards whose connection changed restart,
// the others keep running. An invalid file changes nothing but the problem
// shown, so that a typo in an editor does not cut the forwards.
func (a *app) reload() {
	store := a.store
	if store == nil {
		// Unreadable until now: this reading may be the first good one
		store = config.NewStore(a.path)
	}
	changed, err := store.Reload()
	if err != nil {
		a.configProblem = err.Error()
		a.refresh()
		return
	}
	a.store, a.configProblem = store, ""
	if changed {
		a.loadKubeconfig(store.Kubeconfig())
		a.manager.Load(store.Forwards())
	}
	a.refresh()
}
