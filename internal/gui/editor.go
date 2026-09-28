package gui

import (
	"slices"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/editor"
	"gsontag.fr/kforward/internal/locale"
)

// Actions are what the edit dialog asks the application to do; their errors
// are shown in the dialog, which stays open.
type Actions struct {
	Save   func(config.Forward) error
	Delete func(uuid string) error
}

// field is an entry of the form with the label showing its problem.
type field struct {
	entry   *gtk.Entry
	problem *gtk.Label
}

// Editor is the dialog editing one forward. It runs on the GTK main loop.
type Editor struct {
	tr      *locale.Translator
	actions Actions
	form    editor.Form

	win       *gtk.Window
	fields    map[editor.Field]*field
	group     *gtk.Entry
	namespace *gtk.Entry
	contexts  []string
	context   *gtk.DropDown
	autoStart *gtk.CheckButton
	failure   *gtk.Label
	confirm   bool
}

// OpenEditor shows the dialog for form, over parent; contexts lists the
// contexts of the kubeconfig.
func OpenEditor(
	parent *Window,
	tr *locale.Translator,
	form editor.Form,
	contexts []string,
	actions Actions,
) {
	e := &Editor{tr: tr, actions: actions, form: form, fields: map[editor.Field]*field{}}

	e.win = gtk.NewWindow()
	e.win.SetTransientFor(&parent.win.Window)
	e.win.SetModal(true)
	e.win.SetDefaultSize(420, -1)
	// The dialog is built again at each opening: nothing to keep once closed
	e.win.SetDestroyWithParent(true)
	if form.UUID == "" {
		e.win.SetTitle(tr.T(msgNewTitle, nil))
	} else {
		e.win.SetTitle(tr.T(msgEditTitle, map[string]any{"Name": form.Name}))
	}

	cancel := gtk.NewButtonWithLabel(tr.T(msgCancel, nil))
	cancel.ConnectClicked(e.win.Close)
	save := gtk.NewButtonWithLabel(tr.T(msgSave, nil))
	save.AddCSSClass("suggested-action")
	save.ConnectClicked(e.save)
	header := gtk.NewHeaderBar()
	header.SetShowTitleButtons(false)
	header.PackStart(cancel)
	header.PackEnd(save)
	e.win.SetTitlebar(header)
	// Enter in any entry saves
	e.win.SetDefaultWidget(save)

	grid := gtk.NewGrid()
	grid.SetRowSpacing(4)
	grid.SetColumnSpacing(12)
	grid.SetMarginTop(12)
	grid.SetMarginBottom(12)
	grid.SetMarginStart(12)
	grid.SetMarginEnd(12)

	row := 0
	addRow := func(label string, widget gtk.Widgetter, problem *gtk.Label) {
		l := gtk.NewLabel(label)
		l.SetXAlign(1)
		grid.Attach(l, 0, row, 1, 1)
		grid.Attach(widget, 1, row, 1, 1)
		row++
		if problem != nil {
			grid.Attach(problem, 1, row, 1, 1)
			row++
		}
	}
	addField := func(f editor.Field, label, text, placeholder string) {
		fl := &field{entry: newEntry(text, placeholder), problem: problemLabel()}
		e.fields[f] = fl
		addRow(label, fl.entry, fl.problem)
	}

	addField(editor.Name, tr.T(msgFieldName, nil), form.Name, "grafana")
	e.group = newEntry(form.Group, "")
	addRow(tr.T(msgFieldGroup, nil), e.group, nil)

	e.contexts, e.context = contextChoice(tr, contexts, form.Context)
	addRow(tr.T(msgFieldContext, nil), e.context, nil)
	e.namespace = newEntry(form.Namespace, tr.T(msgDefaultNS, nil))
	addRow(tr.T(msgFieldNamespace, nil), e.namespace, nil)

	addField(editor.Target, tr.T(msgFieldTarget, nil), form.Target, "svc/grafana")
	addField(editor.LocalPort, tr.T(msgFieldLocal, nil), form.LocalPort, "3000")
	addField(editor.RemotePort, tr.T(msgFieldRemote, nil), form.RemotePort, "80, http")
	addField(editor.Address, tr.T(msgFieldAddress, nil), form.Address, "127.0.0.1")
	addField(editor.URL, tr.T(msgFieldURL, nil), form.URL, "http://localhost:3000")

	e.autoStart = gtk.NewCheckButtonWithLabel(tr.T(msgFieldAutoStart, nil))
	e.autoStart.SetActive(form.AutoStart)
	grid.Attach(e.autoStart, 1, row, 1, 1)
	row++

	// Errors of the save itself, such as a file that cannot be written
	e.failure = problemLabel()
	grid.Attach(e.failure, 0, row, 2, 1)
	row++

	if form.UUID != "" {
		remove := gtk.NewButtonWithLabel(tr.T(msgDelete, nil))
		remove.AddCSSClass("destructive-action")
		remove.SetHAlign(gtk.AlignStart)
		remove.ConnectClicked(func() { e.delete(remove) })
		grid.Attach(remove, 0, row, 2, 1)
	}

	e.win.SetChild(grid)
	e.win.Present()
}

// contextChoice returns the drop-down of the contexts, the current context
// first; a context unknown to the kubeconfig is kept, so as not to lose it.
func contextChoice(
	tr *locale.Translator,
	contexts []string,
	selected string,
) ([]string, *gtk.DropDown) {
	values := append([]string{""}, contexts...)
	if selected != "" && !slices.Contains(values, selected) {
		values = append(values, selected)
	}
	labels := make([]string, len(values))
	for i, v := range values {
		labels[i] = v
		if v == "" {
			labels[i] = tr.T(msgCurrentContext, nil)
		}
	}
	dd := gtk.NewDropDownFromStrings(labels)
	for i, v := range values {
		if v == selected {
			dd.SetSelected(uint(i))
		}
	}
	return values, dd
}

func (e *Editor) save() {
	form := e.form
	form.Name = e.fields[editor.Name].entry.Text()
	form.Group = e.group.Text()
	form.Context = e.contexts[e.context.Selected()]
	form.Namespace = e.namespace.Text()
	form.Target = e.fields[editor.Target].entry.Text()
	form.LocalPort = e.fields[editor.LocalPort].entry.Text()
	form.RemotePort = e.fields[editor.RemotePort].entry.Text()
	form.Address = e.fields[editor.Address].entry.Text()
	form.URL = e.fields[editor.URL].entry.Text()
	form.AutoStart = e.autoStart.Active()

	f, problems := form.Forward(e.tr)
	for id, fl := range e.fields {
		showProblem(fl, problems[id])
	}
	if problems != nil {
		return
	}
	if err := e.actions.Save(f); err != nil {
		e.failure.SetText(err.Error())
		e.failure.SetVisible(true)
		return
	}
	e.win.Close()
}

// delete asks for a second click before deleting: there is no undo.
func (e *Editor) delete(button *gtk.Button) {
	if !e.confirm {
		e.confirm = true
		button.SetLabel(e.tr.T(msgConfirmDelete, nil))
		return
	}
	if err := e.actions.Delete(e.form.UUID); err != nil {
		e.failure.SetText(err.Error())
		e.failure.SetVisible(true)
		return
	}
	e.win.Close()
}

func showProblem(fl *field, problem string) {
	fl.problem.SetText(problem)
	fl.problem.SetVisible(problem != "")
	if problem != "" {
		fl.entry.AddCSSClass("error")
	} else {
		fl.entry.RemoveCSSClass("error")
	}
}

func newEntry(text, placeholder string) *gtk.Entry {
	e := gtk.NewEntry()
	e.SetText(text)
	e.SetPlaceholderText(placeholder)
	e.SetActivatesDefault(true)
	e.SetHExpand(true)
	return e
}

func problemLabel() *gtk.Label {
	l := label("error")
	l.SetWrap(true)
	l.SetVisible(false)
	return l
}
