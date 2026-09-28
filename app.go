package main

import (
	"errors"
	"log"
	"slices"
	"strings"

	"fyne.io/systray"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/editor"
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
	tr      *locale.Translator
	// store is nil when the file could not be read: saving would replace it
	store  *config.Store
	client *kube.Client
}

// errUnreadable refuses to save over a configuration file that could not be
// read: its forwards would be lost.
var errUnreadable = errors.New("the configuration file could not be read: fix it by hand first")

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

	forwards, problem := a.load()
	a.problem = problem

	a.tr = locale.FromEnvironment()
	a.tray = tray.New(a.tr, a.trayState, post, a.handle)
	a.window = gui.New(a.gtk, a.tr, a.windowState, post, a.toggle, a.edit)
	a.manager = manager.New(connectorFactory(a.client), forward.DefaultPolicy, func() {
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
func (a *app) load() ([]config.Forward, string) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, err.Error()
	}
	a.path = path

	store := config.NewStore(path)
	if err := store.Load(); err != nil {
		return nil, err.Error()
	}
	a.store = store
	client, err := kube.NewClient(store.Kubeconfig())
	if err != nil {
		return store.Forwards(), err.Error()
	}
	a.client = client
	return store.Forwards(), ""
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

// edit opens the edit dialog; an empty UUID adds a forward.
func (a *app) edit(uuid string) {
	form := editor.Form{}
	if uuid != "" {
		i := slices.IndexFunc(a.manager.Snapshot(), func(e manager.Entry) bool {
			return e.Forward.UUID == uuid
		})
		if i < 0 {
			return
		}
		form = editor.FromForward(a.manager.Snapshot()[i].Forward)
	}
	var contexts []string
	if a.client != nil {
		contexts = a.client.Contexts()
	}
	gui.OpenEditor(a.window, a.tr, form, contexts, gui.Actions{Save: a.save, Delete: a.remove})
}

func (a *app) save(f config.Forward) error {
	if a.store == nil {
		return errUnreadable
	}
	if _, err := a.store.SaveForward(f); err != nil {
		return err
	}
	// Only the forwards whose connection changed restart
	a.manager.Load(a.store.Forwards())
	return nil
}

func (a *app) remove(uuid string) error {
	if a.store == nil {
		return errUnreadable
	}
	if err := a.store.DeleteForward(uuid); err != nil {
		return err
	}
	a.manager.Load(a.store.Forwards())
	return nil
}
