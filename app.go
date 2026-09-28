package main

import (
	"errors"
	"log"
	"strings"

	"fyne.io/systray"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/kube"
	"gsontag.fr/kforward/internal/locale"
	"gsontag.fr/kforward/internal/manager"
	"gsontag.fr/kforward/internal/tray"
)

// app ties the forwards manager to the tray, on the GTK main loop: every
// field is only used from it.
type app struct {
	gtk     *gtk.Application
	started bool
	path    string
	problem string
	manager *manager.Manager
	tray    *tray.Tray
	endTray func()
}

// post runs f on the GTK main loop: the only thread touching the UI.
func post(f func()) {
	glib.IdleAdd(f)
}

func (a *app) activate() {
	// A second launch activates the running instance: nothing to redo
	if a.started {
		return
	}
	a.started = true
	// Without a window, the application would quit at once
	a.gtk.Hold()

	forwards, client, problem := a.load()
	a.problem = problem

	a.tray = tray.New(locale.FromEnvironment(), a.snapshot, post, a.handle)
	a.manager = manager.New(connectorFactory(client), forward.DefaultPolicy, a.tray.Refresh)
	a.manager.Load(forwards)

	start, end := systray.RunWithExternalLoop(a.tray.Refresh, nil)
	start()
	a.endTray = end
}

func (a *app) shutdown() {
	if a.manager != nil {
		a.manager.Close()
	}
	if a.endTray != nil {
		a.endTray()
	}
}

// load reads the configuration and the kubeconfig. Their errors do not stop
// the application: they are shown at the top of the menu.
func (a *app) load() ([]config.Forward, *kube.Client, string) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, nil, firstLine(err)
	}
	a.path = path

	store := config.NewStore(path)
	if err := store.Load(); err != nil {
		return nil, nil, firstLine(err)
	}
	client, err := kube.NewClient(store.Kubeconfig())
	if err != nil {
		return store.Forwards(), nil, firstLine(err)
	}
	return store.Forwards(), client, ""
}

// connectorFactory builds the connectors through client; without one, every
// start fails with the reason.
func connectorFactory(client *kube.Client) manager.ConnectorFactory {
	return func(f config.Forward) (forward.Connector, error) {
		if client == nil {
			return nil, errors.New("no usable kubeconfig")
		}
		c, err := forward.NewClusterConnector(client, f)
		if err != nil {
			// A nil *ClusterConnector in a Connector interface would not be nil
			return nil, err
		}
		return c, nil
	}
}

func (a *app) snapshot() ([]manager.Entry, string) {
	return a.manager.Snapshot(), a.problem
}

func (a *app) handle(it tray.Item) {
	var err error
	switch it.Action.Op {
	case tray.ToggleForward:
		if it.Checked {
			err = a.manager.Stop(it.Action.UUID)
		} else {
			err = a.manager.Start(it.Action.UUID)
		}
	case tray.OpenURL:
		err = gio.AppInfoLaunchDefaultForURI(it.Action.URL, nil)
	case tray.StopAll:
		a.manager.StopAll()
	case tray.EditConfig:
		err = gio.AppInfoLaunchDefaultForURI(gio.NewFileForPath(a.path).URI(), nil)
	case tray.Quit:
		a.gtk.Quit()
	case tray.None:
	}
	if err != nil {
		log.Printf("%s: %v", it.Text, err)
	}
}

// firstLine keeps a menu item on one line: validation errors span several.
func firstLine(err error) string {
	line, _, _ := strings.Cut(err.Error(), "\n")
	return line
}
