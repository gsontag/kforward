package gui

import (
	"slices"
	"strconv"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/editor"
	"github.com/gsontag/kforward/internal/locale"
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

	// suggester is nil without a usable kubeconfig: the fields stay plain
	suggester   *editor.Suggester
	suggestions map[editor.Level]*suggestions
}

// OpenEditor shows the dialog for form, over parent; contexts lists the
// contexts of the kubeconfig.
func OpenEditor(
	parent *Window,
	tr *locale.Translator,
	form editor.Form,
	contexts []string,
	src editor.Source,
	post func(func()),
	actions Actions,
) {
	e := &Editor{
		tr:          tr,
		actions:     actions,
		form:        form,
		fields:      map[editor.Field]*field{},
		suggestions: map[editor.Level]*suggestions{},
	}

	if src != nil {
		e.suggester = editor.NewSuggester(src, post, e.suggest)
	}

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
	addField := func(f editor.Field, label, text, placeholder string) *gtk.Entry {
		fl := &field{entry: newEntry(text, placeholder), problem: problemLabel()}
		e.fields[f] = fl
		addRow(label, e.suggestible(fl.entry, f), fl.problem)
		return fl.entry
	}

	addField(editor.Name, tr.T(msgFieldName, nil), form.Name, "grafana")
	e.group = newEntry(form.Group, "")
	addRow(tr.T(msgFieldGroup, nil), e.group, nil)

	e.contexts, e.context = contextChoice(tr, contexts, form.Context)
	addRow(tr.T(msgFieldContext, nil), e.context, nil)
	e.namespace = newEntry(form.Namespace, tr.T(msgDefaultNS, nil))
	addRow(tr.T(msgFieldNamespace, nil), e.suggestible(e.namespace, namespaceField), nil)

	target := addField(editor.Target, tr.T(msgFieldTarget, nil), form.Target, "svc/grafana")
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

	if e.suggester != nil {
		// Each change queries the fields after it; the first queries are for the
		// values of the forward being edited
		e.context.NotifyProperty(
			"selected",
			func() { e.suggester.SetContext(e.contexts[e.context.Selected()]) },
		)
		e.namespace.ConnectChanged(func() { e.suggester.SetNamespace(e.namespace.Text()) })
		target.ConnectChanged(func() { e.suggester.SetTarget(target.Text()) })
		e.suggester.SetContext(form.Context)
		e.suggester.SetNamespace(form.Namespace)
		e.suggester.SetTarget(form.Target)
		e.win.ConnectDestroy(e.suggester.Close)
	}

	e.win.SetChild(grid)
	e.win.Present()
}

// namespaceField stands for the namespace, which has suggestions but no
// problem of its own.
const namespaceField editor.Field = -1

// levels maps the fields with suggestions to the level that feeds them.
var levels = map[editor.Field]editor.Level{
	namespaceField:    editor.Namespaces,
	editor.Target:     editor.Targets,
	editor.RemotePort: editor.Ports,
}

// suggestible adds a suggestions button to entry when field has some and the
// cluster can be asked.
func (e *Editor) suggestible(entry *gtk.Entry, f editor.Field) gtk.Widgetter {
	level, ok := levels[f]
	if !ok || e.suggester == nil {
		return entry
	}
	box, s := withSuggestions(e.tr, entry, func(c editor.Choice) { e.pick(f, entry, c) })
	e.suggestions[level] = s
	return box
}

// pick fills entry with a chosen suggestion.
func (e *Editor) pick(f editor.Field, entry *gtk.Entry, c editor.Choice) {
	entry.SetText(c.Value)
	local := e.fields[editor.LocalPort].entry
	if f == editor.RemotePort && local.Text() == "" {
		local.SetText(strconv.Itoa(int(editor.LocalPortFor(c.Port))))
	}
}

// suggest shows the suggestions delivered for a level.
func (e *Editor) suggest(l editor.Level, choices []editor.Choice, err error) {
	if s, ok := e.suggestions[l]; ok {
		s.set(choices, err)
	}
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
