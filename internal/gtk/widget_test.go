package gtk

import "testing"

func TestLabel(t *testing.T) {
	var text string
	onMain(t, func() {
		l := NewLabel("before")
		defer l.obj.release()
		l.SetText("after")
		text = l.Text()
	})
	check(t, "text", text, "after")
}

func TestButtonClicked(t *testing.T) {
	var label string
	clicks := 0
	onMain(t, func() {
		b := NewButtonWithLabel("Save")
		defer b.obj.release()
		b.ConnectClicked(func() { clicks++ })
		b.obj.emit("clicked")
		b.obj.emit("clicked")
		b.SetLabel("Delete")
		label = b.Label()
	})
	check(t, "clicks", clicks, 2)
	check(t, "label", label, "Delete")
}

func TestVisible(t *testing.T) {
	var before, after bool
	onMain(t, func() {
		l := NewLabel("")
		defer l.obj.release()
		before = l.IsVisible()
		l.SetVisible(false)
		after = l.IsVisible()
	})
	// A widget is visible by default in GTK 4
	check(t, "before", before, true)
	check(t, "after", after, false)
}

// GTK owns the widgets: a child lives as long as its parent. Its signal
// handlers, even one holding the Go value of the child, must not keep it
// alive, or every widget ever created would leak.
func TestChildFreedWithItsParent(t *testing.T) {
	before := live.Load()
	var aliveBefore, aliveAfter bool
	var label string
	onMain(t, func() {
		box := NewBox(OrientationVertical, 0)
		button := NewButtonWithLabel("Save")
		// The handler holds the button: a cycle, if Go owned the widget
		button.ConnectClicked(func() { button.SetLabel("saved") })
		box.Append(button)
		aliveBefore = button.obj.alive()

		box.obj.release()
		aliveAfter = button.obj.alive()
		// Gone: the methods do nothing, rather than touch freed memory
		button.SetLabel("too late")
		label = button.Label()
	})
	check(t, "alive in its box", aliveBefore, true)
	check(t, "alive after its box", aliveAfter, false)
	check(t, "label once gone", label, "")
	check(t, "handles not released", live.Load()-before, int64(0))
}
