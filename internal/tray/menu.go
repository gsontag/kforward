// Package tray shows the forwards as switches in the menu of a status
// notifier icon.
package tray

import (
	"fmt"

	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/manager"
)

// Kind is the kind of a menu item.
type Kind int

// Menu item kinds.
const (
	Separator Kind = iota
	Label
	Toggle
	Button
	Submenu
)

// Op is what a click on a menu item does.
type Op int

// Menu operations.
const (
	None Op = iota
	ToggleForward
	OpenURL
	StopAll
	EditConfig
	Quit
)

// Action is the operation of a click, with its argument.
type Action struct {
	Op Op
	// UUID is the forward of ToggleForward, URL the address of OpenURL.
	UUID, URL string
}

// Item is one entry of the menu: plain data, built without any D-Bus call,
// so that the menu content can be tested on its own.
type Item struct {
	Kind     Kind
	Text     string
	Tooltip  string
	Checked  bool
	Disabled bool
	Action   Action
	Children []Item
}

const ungrouped = "Divers"

// Build returns the menu showing entries; problem, when not empty, is a
// configuration error displayed at the top.
func Build(entries []manager.Entry, problem string) []Item {
	var items []Item
	if problem != "" {
		items = append(
			items,
			Item{Kind: Label, Text: "⚠ " + problem, Disabled: true},
			Item{Kind: Separator},
		)
	}
	if len(entries) == 0 && problem == "" {
		items = append(
			items,
			Item{Kind: Label, Text: "Aucun forward configuré", Disabled: true},
			Item{Kind: Separator},
		)
	}

	items = append(items, forwardItems(entries)...)
	if len(entries) > 0 {
		items = append(items, Item{Kind: Separator})
	}

	if urls := urlItems(entries); len(urls) > 0 {
		items = append(
			items,
			Item{Kind: Submenu, Text: "Ouvrir dans le navigateur", Children: urls},
		)
	}
	items = append(
		items,
		Item{
			Kind:     Button,
			Text:     "Tout couper",
			Disabled: !anyRunning(entries),
			Action:   Action{Op: StopAll},
		},
		Item{Kind: Separator},
		Item{Kind: Button, Text: "Editer la configuration...", Action: Action{Op: EditConfig}},
		Item{Kind: Separator},
		Item{Kind: Button, Text: "Quitter", Action: Action{Op: Quit}},
	)
	return items
}

// forwardItems lists the switches, under a header per group when the
// configuration uses groups; entries come sorted by the manager, so each
// group is contiguous.
func forwardItems(entries []manager.Entry) []Item {
	grouped := false
	for _, e := range entries {
		if e.Forward.Group != "" {
			grouped = true
			break
		}
	}

	var items []Item
	for i, e := range entries {
		group := e.Forward.Group
		if grouped && (i == 0 || entries[i-1].Forward.Group != group) {
			if i > 0 {
				items = append(items, Item{Kind: Separator})
			}
			if group == "" {
				group = ungrouped
			}
			items = append(items, Item{Kind: Label, Text: group, Disabled: true})
		}
		items = append(items, toggleItem(e))
	}
	return items
}

func toggleItem(e manager.Entry) Item {
	f, s := e.Forward, e.Status
	text := fmt.Sprintf("%s  :%d", f.Name, f.LocalPort)
	switch s.State {
	case forward.Connecting:
		text += " - connexion..."
	case forward.Failed:
		text += " - échec"
	case forward.Stopped, forward.Active:
	}

	tooltip := fmt.Sprintf("%s → localhost:%d", f.Target, f.LocalPort)
	if s.Err != nil && s.State != forward.Active {
		tooltip = s.Err.Error()
	}
	return Item{
		Kind:    Toggle,
		Text:    text,
		Tooltip: tooltip,
		Checked: s.State == forward.Connecting || s.State == forward.Active,
		Action:  Action{Op: ToggleForward, UUID: f.UUID},
	}
}

func urlItems(entries []manager.Entry) []Item {
	var items []Item
	for _, e := range entries {
		if e.Forward.URL == "" {
			continue
		}
		items = append(items, Item{
			Kind:     Button,
			Text:     e.Forward.Name,
			Tooltip:  e.Forward.URL,
			Disabled: e.Status.State != forward.Active,
			Action:   Action{Op: OpenURL, URL: e.Forward.URL},
		})
	}
	return items
}

func anyRunning(entries []manager.Entry) bool {
	for _, e := range entries {
		if s := e.Status.State; s == forward.Connecting || s == forward.Active {
			return true
		}
	}
	return false
}

// Summary is what the icon itself shows.
type Summary struct {
	// Active counts the forwards ready to use.
	Active int
	// Busy is true while at least one forward runs or connects.
	Busy bool
}

// Summarize counts the running forwards.
func Summarize(entries []manager.Entry) Summary {
	var s Summary
	for _, e := range entries {
		switch e.Status.State {
		case forward.Active:
			s.Active++
			s.Busy = true
		case forward.Connecting:
			s.Busy = true
		case forward.Stopped, forward.Failed:
		}
	}
	return s
}
