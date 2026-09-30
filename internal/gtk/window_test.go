package gtk

import (
	"fmt"
	"testing"
)

func TestWindowHideOnClose(t *testing.T) {
	var title string
	var visible []bool
	var shown, hidden, alive bool
	onMain(t, func() {
		w := NewWindow()
		w.SetTitle("Kube Forwarder")
		w.SetDefaultSize(560, 480)
		w.SetHideOnClose(true)
		w.NotifyProperty("visible", func() { visible = append(visible, w.IsVisible()) })
		title = w.Title()

		w.Present()
		shown = w.IsVisible()
		w.Close()
		hidden, alive = !w.IsVisible(), w.obj.alive()
		// Destroyed for good at the end of the test
		w.SetHideOnClose(false)
		w.Close()
	})
	check(t, "title", title, "Kube Forwarder")
	check(t, "shown", shown, true)
	check(t, "hidden by close", hidden, true)
	check(t, "alive after close", alive, true)
	check(t, "visible notifications", fmt.Sprint(visible), "[true false]")
}

// A closed window is destroyed with its children: nothing leaks, the
// handlers of the children included.
func TestWindowCloseDestroys(t *testing.T) {
	before := live.Load()
	var windowAlive, childAlive, barAlive bool
	onMain(t, func() {
		w := NewWindow()
		bar := NewHeaderBar()
		bar.SetShowTitleButtons(false)
		add := NewButtonFromIconName("list-add-symbolic")
		add.ConnectClicked(func() { add.SetLabel("clicked") })
		bar.PackStart(add)
		bar.PackEnd(NewButtonWithLabel("Save"))
		w.SetTitlebar(bar)
		child := NewLabel("content")
		w.SetChild(child)
		w.NotifyProperty("visible", func() {})
		w.Present()

		w.Close()
		windowAlive, childAlive, barAlive = w.obj.alive(), child.obj.alive(), bar.obj.alive()
	})
	check(t, "window alive", windowAlive, false)
	check(t, "child alive", childAlive, false)
	check(t, "header bar alive", barAlive, false)
	check(t, "handles not released", live.Load()-before, int64(0))
}
