package main

import (
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"

	"fyne.io/systray"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gsontag/kforward/internal/autostart"
	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/editor"
	"github.com/gsontag/kforward/internal/forward"
	"github.com/gsontag/kforward/internal/gui"
	"github.com/gsontag/kforward/internal/kube"
	"github.com/gsontag/kforward/internal/locale"
	"github.com/gsontag/kforward/internal/manager"
	"github.com/gsontag/kforward/internal/notify"
	"github.com/gsontag/kforward/internal/tray"
)

// app ties the forwards manager to the tray, on the GTK main loop: every
// field is only used from it.
type app struct {
	gtk     *gtk.Application
	started bool
	// background starts in the tray only, as at login
	background bool
	path       string
	// configProblem and kubeProblem are the errors of the configuration file
	// and of the kubeconfig, shown in the menu and the window
	configProblem, kubeProblem string
	manager                    *manager.Manager
	tray                       *tray.Tray
	endTray                    func()
	window                     *gui.Window
	tr                         *locale.Translator
	// store is nil when the file could not be read: saving would replace it
	store  *config.Store
	client *kube.Client
	// kubeconfig is the path client was loaded from
	kubeconfig string

	monitor     *gio.FileMonitor
	reloadTimer glib.SourceHandle

	tracker *notify.Tracker
	sender  *notify.Sender
	// notifyPending coalesces the checks for notifications, like tray.Refresh
	notifyPending atomic.Bool

	// autostart is the entry starting the application at login
	autostart autostart.Entry

	// logPath is the log file, empty when the log only goes to stderr
	logPath string
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

	a.logPath = setupLog()

	slog.Info("start", "version", version())

	a.autostart = newAutostart()

	forwards := a.load()

	a.tr = locale.FromEnvironment()
	a.tray = tray.New(a.tr, a.trayState, post, a.handle)
	a.window = gui.New(a.gtk, a.tr, a.windowState, post, a.toggle, a.edit)
	a.manager = manager.New(a.connector, forward.DefaultPolicy, slog.Default(), a.refresh)
	a.manager.Load(forwards)
	a.watchConfig()

	a.tracker = notify.NewTracker(a.tr)
	a.sender = notify.NewSender(appName, appID, "network-error-symbolic")

	start, end := systray.RunWithExternalLoop(a.tray.Refresh, nil)
	start()
	a.endTray = end

	if !a.background {
		a.window.Show()
	}
}

// newAutostart returns the autostart entry of the running program. Without
// one, the zero entry is never enabled and fails to enable: the menu item
// then only logs why.
func newAutostart() autostart.Entry {
	e, err := autostart.Default(appID, appName)
	if err != nil {
		slog.Warn("autostart", "err", err)
	}
	return e
}

func (a *app) shutdown() {
	if a.manager != nil {
		a.manager.Close()
	}
	if a.endTray != nil {
		a.endTray()
	}
	if a.sender != nil {
		a.sender.Close()
	}
}

// load reads the configuration and the kubeconfig. Their errors do not stop
// the application: they are shown at the top of the menu and in the window.
func (a *app) load() []config.Forward {
	path, err := config.DefaultPath()
	if err != nil {
		a.configProblem = err.Error()
		return nil
	}
	a.path = path

	store := config.NewStore(path)
	if err := store.Load(); err != nil {
		a.configProblem = err.Error()
		return nil
	}
	a.store = store
	a.loadKubeconfig(store.Kubeconfig())

	return store.Forwards()
}

// loadKubeconfig loads the kubeconfig at path, unless it is already loaded.
func (a *app) loadKubeconfig(path string) {
	if a.client != nil && path == a.kubeconfig {
		return
	}
	client, err := kube.NewClient(path)
	if err != nil {
		a.client, a.kubeProblem = nil, err.Error()
		return
	}
	a.client, a.kubeconfig, a.kubeProblem = client, path, ""
}

// connector builds the connector of f through the current kubeconfig: a
// forward started after a change of kubeconfig uses the new one.
func (a *app) connector(f config.Forward) (forward.Connector, error) {
	if a.client == nil {
		return nil, errors.New("no usable kubeconfig")
	}
	c, err := forward.NewClusterConnector(a.client, f)
	if err != nil {
		// A nil *ClusterConnector in a Connector interface would not be nil
		return nil, err
	}
	return c, nil
}

// refresh updates the tray and the window, and notifies what went wrong;
// safe from any goroutine.
func (a *app) refresh() {
	a.tray.Refresh()
	a.window.Refresh()
	if !a.notifyPending.Swap(true) {
		post(a.notify)
	}
}

func (a *app) notify() {
	a.notifyPending.Store(false)
	for _, n := range a.tracker.Update(a.manager.Snapshot(), a.problem()) {
		a.sender.Send(n)
	}
}

// problem is what is wrong with the configuration or the kubeconfig.
func (a *app) problem() string {
	var lines []string
	for _, p := range []string{a.configProblem, a.kubeProblem} {
		if p != "" {
			lines = append(lines, p)
		}
	}
	return strings.Join(lines, "\n")
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
	case tray.OpenLog:
		err = gio.AppInfoLaunchDefaultForURI(gio.NewFileForPath(a.logPath).URI(), nil)
	case tray.ToggleAutostart:
		if it.Checked {
			err = a.autostart.Disable()
		} else {
			err = a.autostart.Enable()
		}
		a.tray.Refresh()
	case tray.None:
	}
	if err != nil {
		slog.Warn("menu action", "item", it.Text, "err", err)
	}
}

func (a *app) trayState() tray.State {
	// A menu item has a single line: the first one tells what is wrong
	line, _, _ := strings.Cut(a.problem(), "\n")
	return tray.State{
		Entries:   a.manager.Snapshot(),
		Problem:   line,
		Autostart: a.autostart.Enabled(),
	}
}

func (a *app) windowState() ([]manager.Entry, string) {
	return a.manager.Snapshot(), a.problem()
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
		slog.Warn("switch", "uuid", uuid, "err", err)
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
	// A nil interface, not an interface holding a nil client: no suggestions
	var src editor.Source
	if a.client != nil {
		contexts = a.client.Contexts()
		src = editor.ClusterSource{Client: a.client}
	}
	gui.OpenEditor(
		a.window,
		a.tr,
		form,
		contexts,
		src,
		post,
		gui.Actions{Save: a.save, Delete: a.remove},
	)
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
