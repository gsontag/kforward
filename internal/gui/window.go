// Package gui holds the GTK widgets. It is kept apart from the models it
// shows: GTK takes minutes to build with the race detector, which the unit
// tests use.
package gui

import (
	"strings"
	"sync/atomic"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"gsontag.fr/kforward/internal/locale"
	"gsontag.fr/kforward/internal/manager"
	"gsontag.fr/kforward/internal/window"
)

// Source provides what the window shows: the forwards and the configuration
// problem, in full, if any.
type Source func() (entries []manager.Entry, problem string)

// Window lists the forwards with a switch each. Except Refresh, its methods
// run on the GTK main loop, where post executes functions.
type Window struct {
	tr     *locale.Translator
	source Source
	post   func(func())
	toggle func(uuid string, on bool)

	pending atomic.Bool

	win     *gtk.ApplicationWindow
	problem *gtk.Label
	list    *gtk.ListBox
	rows    []window.Row
	widgets []rowWidgets
	layout  string
	// updating tells the switch handlers that the change comes from render
	updating bool
}

type rowWidgets struct {
	title, subtitle, status, err *gtk.Label
	sw                           *gtk.Switch
}

// New builds the window, hidden; toggle receives the switch changes made by
// the user.
func New(
	app *gtk.Application,
	tr *locale.Translator,
	source Source,
	post func(func()),
	toggle func(string, bool),
) *Window {
	w := &Window{tr: tr, source: source, post: post, toggle: toggle}

	w.win = gtk.NewApplicationWindow(app)
	w.win.SetTitle(tr.T(msgTitle, nil))
	w.win.SetDefaultSize(560, 480)
	// Closing only hides: the forwards keep running behind the tray icon
	w.win.SetHideOnClose(true)

	w.problem = gtk.NewLabel("")
	w.problem.AddCSSClass("error")
	w.problem.SetWrap(true)
	w.problem.SetXAlign(0)
	w.problem.SetVisible(false)

	w.list = gtk.NewListBox()
	w.list.SetSelectionMode(gtk.SelectionNone)
	w.list.AddCSSClass("rich-list")
	w.list.SetPlaceholder(gtk.NewLabel(tr.T(msgEmpty, nil)))
	w.list.SetHeaderFunc(w.header)

	scroll := gtk.NewScrolledWindow()
	scroll.SetChild(w.list)
	scroll.SetVExpand(true)

	box := gtk.NewBox(gtk.OrientationVertical, 6)
	box.Append(w.problem)
	box.Append(scroll)
	w.win.SetChild(box)

	// A hidden window is not rendered: catch up when it shows again
	w.win.NotifyProperty("visible", func() {
		if w.win.IsVisible() {
			w.render()
		}
	})
	return w
}

// Show brings the window to the front.
func (w *Window) Show() {
	w.win.Present()
}

// Refresh schedules an update, like tray.Refresh: safe from any goroutine,
// and coalesced.
func (w *Window) Refresh() {
	if w.pending.Swap(true) {
		return
	}
	w.post(func() {
		w.pending.Store(false)
		if w.win.IsVisible() {
			w.render()
		}
	})
}

func (w *Window) render() {
	entries, problem := w.source()
	w.problem.SetText(problem)
	w.problem.SetVisible(problem != "")

	// Before rebuild: appending a row already calls the header function
	w.rows = window.Rows(entries, w.tr)
	if l := layout(w.rows); l != w.layout {
		w.rebuild(w.rows)
		w.layout = l
	}

	w.updating = true
	defer func() { w.updating = false }()
	for i, r := range w.rows {
		ws := w.widgets[i]
		ws.title.SetText(r.Title)
		ws.subtitle.SetText(r.Subtitle)
		ws.status.SetText(r.Status)
		ws.err.SetText(r.Error)
		ws.err.SetVisible(r.Error != "")
		// Position first, then the "on" state: they differ while connecting
		ws.sw.SetActive(r.Wanted)
		ws.sw.SetState(r.Active)
	}
	// Headers depend on the groups, which may have changed in place
	w.list.InvalidateHeaders()
}

func (w *Window) rebuild(rows []window.Row) {
	w.list.RemoveAll()
	w.widgets = w.widgets[:0]
	for _, r := range rows {
		ws := rowWidgets{
			title:    label("heading"),
			subtitle: label("dim-label"),
			status:   label("dim-label"),
			err:      label("error"),
			sw:       gtk.NewSwitch(),
		}
		ws.err.SetWrap(true)
		ws.sw.SetVAlign(gtk.AlignCenter)
		uuid := r.UUID
		ws.sw.ConnectStateSet(func(state bool) bool {
			if w.updating {
				return false
			}
			w.toggle(uuid, state)
			// The manager sets the state once the forward is active
			return true
		})

		texts := gtk.NewBox(gtk.OrientationVertical, 2)
		texts.SetHExpand(true)
		texts.Append(ws.title)
		texts.Append(ws.subtitle)
		texts.Append(ws.status)
		texts.Append(ws.err)

		line := gtk.NewBox(gtk.OrientationHorizontal, 12)
		line.SetMarginTop(6)
		line.SetMarginBottom(6)
		line.SetMarginStart(12)
		line.SetMarginEnd(12)
		line.Append(texts)
		line.Append(ws.sw)

		w.list.Append(line)
		w.widgets = append(w.widgets, ws)
	}
}

func (w *Window) header(row, before *gtk.ListBoxRow) {
	group := w.rows[row.Index()].Group
	if group == "" || (before != nil && w.rows[before.Index()].Group == group) {
		row.SetHeader(nil)
		return
	}
	h := label("heading")
	h.SetText(group)
	h.SetMarginTop(12)
	h.SetMarginStart(12)
	row.SetHeader(h)
}

func label(class string) *gtk.Label {
	l := gtk.NewLabel("")
	l.SetXAlign(0)
	l.AddCSSClass(class)
	return l
}

// layout identifies the rows: the widgets are rebuilt only when it changes.
func layout(rows []window.Row) string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.UUID
	}
	return strings.Join(ids, ",")
}
