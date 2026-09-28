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
	"gsontag.fr/kforward/internal/gui"
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
	window  *gui.Window
}

// post runs f on the GTK main loop: the only thread touching the UI.
func post(f func()) {
	glib.IdleAdd(f)
}

func (a *app) activate() {
	// A second launch activates the running instance: show its window
	if a.started {
		a.window.Show()
		return
	}
	a.started = true
	// Without a window, the application would quit at once
	a.gtk.Hold()

	forwards, client, problem := a.load()
	a.problem = problem

	tr := locale.FromEnvironment()
	a.tray = tray.New(tr, a.trayState, post, a.handle)
	a.window = gui.New(a.gtk, tr, a.windowState, post, a.toggle)
	a.manager = manager.New(connectorFactory(client), forward.DefaultPolicy, func() {
		a.tray.Refresh()
		a.window.Refresh()
	})
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
// the application: they are shown at the top of the menu and in the window.
func (a *app) load() ([]config.Forward, *kube.Client, string) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, nil, err.Error()
	}
	a.path = path

	store := config.NewStore(path)
	if err := store.Load(); err != nil {
		return nil, nil, err.Error()
	}
	client, err := kube.NewClient(store.Kubeconfig())
	if err != nil {
		return store.Forwards(), nil, err.Error()
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
	case tray.ShowWindow:
		a.window.Show()
	case tray.None:
	}
	if err != nil {
		log.Printf("%s: %v", it.Text, err)
	}
}

func (a *app) trayState() ([]manager.Entry, string) {
	// A menu item has a single line: the first one tells what is wrong
	line, _, _ := strings.Cut(a.problem, "\n")
	return a.manager.Snapshot(), line
}

func (a *app) windowState() ([]manager.Entry, string) {
	return a.manager.Snapshot(), a.problem
}

// toggle applies a switch of the window.
func (a *app) toggle(uuid string, on bool) {
	var err error
	if on {
		err = a.manager.Start(uuid)
	} else {
		err = a.manager.Stop(uuid)
	}
	if err != nil {
		log.Printf("%s: %v", uuid, err)
	}
}
