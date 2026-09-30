package gtk

import (
	"fmt"
	"testing"
)

func TestEntry(t *testing.T) {
	var text string
	var changes []string
	onMain(t, func() {
		e := NewEntry()
		defer e.obj.release()
		e.SetPlaceholderText("svc/grafana")
		e.SetActivatesDefault(true)
		e.ConnectChanged(func() { changes = append(changes, e.Text()) })
		e.SetText("s")
		e.SetText("svc")
		text = e.Text()
	})
	check(t, "text", text, "svc")
	// Replacing a text empties the entry first
	check(t, "changes", fmt.Sprintf("%q", changes), `["s" "" "svc"]`)
}

func TestDropDown(t *testing.T) {
	var first, selected uint
	var notified int
	onMain(t, func() {
		d := NewDropDownFromStrings([]string{"(current)", "prod", "staging"})
		defer d.obj.release()
		first = d.Selected()
		d.NotifyProperty("selected", func() { notified++ })
		d.SetSelected(2)
		selected = d.Selected()
	})
	check(t, "selected at first", first, uint(0))
	check(t, "selected", selected, uint(2))
	check(t, "notifications", notified, 1)
}

func TestCheckButton(t *testing.T) {
	var before, after bool
	onMain(t, func() {
		c := NewCheckButtonWithLabel("Start with the application")
		defer c.obj.release()
		before = c.Active()
		c.SetActive(true)
		after = c.Active()
	})
	check(t, "before", before, false)
	check(t, "after", after, true)
}

// The suggestions button: its popover, holding a list, belongs to it.
func TestMenuButtonOwnsItsPopover(t *testing.T) {
	var popoverAlive, listAlive, popoverAfter, listAfter bool
	onMain(t, func() {
		b := NewMenuButton()
		b.SetIconName("pan-down-symbolic")
		p := NewPopover()
		l := NewListBox()
		p.SetChild(l)
		b.SetPopover(p)
		p.Popdown()
		popoverAlive, listAlive = p.obj.alive(), l.obj.alive()

		b.obj.release()
		popoverAfter, listAfter = p.obj.alive(), l.obj.alive()
	})
	check(t, "popover alive", popoverAlive, true)
	check(t, "list alive", listAlive, true)
	check(t, "popover after its button", popoverAfter, false)
	check(t, "list after its button", listAfter, false)
}

// The edit dialog: a grid in a modal window over the main one, destroyed
// with it.
func TestDialogDestroyedWithItsParent(t *testing.T) {
	before := live.Load()
	var dialogAlive, gridAlive bool
	onMain(t, func() {
		parent := NewWindow()
		parent.Present()

		dialog := NewWindow()
		dialog.SetTransientFor(parent)
		dialog.SetModal(true)
		dialog.SetDestroyWithParent(true)
		grid := NewGrid()
		grid.SetRowSpacing(4)
		grid.SetColumnSpacing(12)
		save := NewButtonWithLabel("Save")
		save.ConnectClicked(dialog.Close)
		entry := NewEntry()
		entry.ConnectChanged(func() { entry.RemoveCSSClass("error") })
		grid.Attach(NewLabel("Name"), 0, 0, 1, 1)
		grid.Attach(entry, 1, 0, 1, 1)
		grid.Attach(save, 0, 1, 2, 1)
		dialog.SetChild(grid)
		dialog.SetDefaultWidget(save)
		dialog.Present()

		parent.Close()
		dialogAlive, gridAlive = dialog.obj.alive(), grid.obj.alive()
	})
	check(t, "dialog alive", dialogAlive, false)
	check(t, "grid alive", gridAlive, false)
	check(t, "handles not released", live.Load()-before, int64(0))
}
