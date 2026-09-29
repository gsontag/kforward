package gui

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/gsontag/kforward/internal/editor"
	"github.com/gsontag/kforward/internal/locale"
)

// suggestions is the button next to a field that lists what the cluster
// proposes for it.
type suggestions struct {
	tr      *locale.Translator
	button  *gtk.MenuButton
	popover *gtk.Popover
	list    *gtk.ListBox
	choices []editor.Choice
}

// withSuggestions returns entry with a suggestions button on its right;
// pick receives the chosen suggestion.
func withSuggestions(
	tr *locale.Translator,
	entry *gtk.Entry,
	pick func(editor.Choice),
) (*gtk.Box, *suggestions) {
	s := &suggestions{
		tr:      tr,
		list:    gtk.NewListBox(),
		popover: gtk.NewPopover(),
		button:  gtk.NewMenuButton(),
	}

	// A click picks a row: no selection to show
	s.list.SetSelectionMode(gtk.SelectionNone)
	s.list.ConnectRowActivated(func(row *gtk.ListBoxRow) {
		s.popover.Popdown()
		pick(s.choices[row.Index()])
	})
	scroll := gtk.NewScrolledWindow()
	scroll.SetChild(s.list)
	// As wide as the longest suggestion, as high as the list up to a limit
	scroll.SetPropagateNaturalWidth(true)
	scroll.SetPropagateNaturalHeight(true)
	scroll.SetMaxContentHeight(300)
	s.popover.SetChild(scroll)

	s.button.SetIconName("pan-down-symbolic")
	s.button.SetPopover(s.popover)
	s.set(nil, nil)

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	// Drawn as a single control, like a combo box
	box.AddCSSClass("linked")
	box.Append(entry)
	box.Append(s.button)
	return box, s
}

// set replaces the suggestions; err explains an empty list.
func (s *suggestions) set(choices []editor.Choice, err error) {
	s.choices = choices
	s.list.RemoveAll()
	for _, c := range choices {
		l := gtk.NewLabel(c.Label)
		l.SetXAlign(0)
		l.SetMarginStart(6)
		l.SetMarginEnd(6)
		s.list.Append(l)
	}
	s.button.SetSensitive(len(choices) > 0)
	switch {
	case err != nil:
		s.button.SetTooltipText(err.Error())
	case len(choices) == 0:
		s.button.SetTooltipText(s.tr.T(msgNoSuggestion, nil))
	default:
		s.button.SetTooltipText(s.tr.T(msgSuggestions, nil))
	}
}
