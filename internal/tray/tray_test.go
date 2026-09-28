package tray

import (
	"runtime"
	"sync"
	"testing"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/locale"
	"gsontag.fr/kforward/internal/manager"
)

func TestShape(t *testing.T) {
	menu := func(items ...Item) []Item { return items }
	toggle := func(text string, checked bool) Item {
		return Item{Kind: Toggle, Text: text, Checked: checked, Action: Action{Op: ToggleForward, UUID: text}}
	}
	base := menu(toggle("a", false), Item{Kind: Separator}, Item{Kind: Button, Text: "Quit"})

	tests := []struct {
		name  string
		other []Item
		same  bool
	}{
		// Texts and states differ: updated in place
		{"texts and states", menu(
			toggle("a — connecting…", true), Item{Kind: Separator}, Item{Kind: Button, Text: "Quitter", Disabled: true},
		), true},
		{"item added", menu(
			toggle("a", false), toggle("b", false), Item{Kind: Separator}, Item{Kind: Button, Text: "Quit"},
		), false},
		{"kind changed", menu(
			Item{Kind: Label, Text: "a"}, Item{Kind: Separator}, Item{Kind: Button, Text: "Quit"},
		), false},
		{"submenu appears", menu(
			toggle("a", false), Item{Kind: Separator}, Item{Kind: Submenu, Children: menu(Item{Kind: Button})},
		), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check(t, "same shape", shape(tt.other) == shape(base), tt.same)
		})
	}
}

func TestShapeSubmenuChildren(t *testing.T) {
	withLinks := func(n int) []Item {
		children := make([]Item, n)
		for i := range children {
			children[i] = Item{Kind: Button}
		}
		return []Item{{Kind: Submenu, Children: children}, {Kind: Button}}
	}
	// One more link in the submenu is a structure change, even though the top
	// level looks the same
	if shape(withLinks(1)) == shape(withLinks(2)) {
		t.Error("a child added to a submenu must change the shape")
	}
	check(t, "same children", shape(withLinks(2)), shape(withLinks(2)))
}

func TestAt(t *testing.T) {
	items := []Item{
		{Kind: Toggle, Text: "a"},
		{Kind: Separator},
		{Kind: Submenu, Text: "links", Children: []Item{{Text: "grafana"}, {Text: "keycloak"}}},
	}
	tests := []struct {
		path   []int
		want   string
		wantOK bool
	}{
		{[]int{0}, "a", true},
		{[]int{2}, "links", true},
		{[]int{2, 1}, "keycloak", true},
		// A click racing a rebuild may point past the new menu
		{[]int{3}, "", false},
		{[]int{2, 5}, "", false},
		{[]int{0, 0}, "", false},
		{nil, "", false},
	}
	for _, tt := range tests {
		it, ok := at(items, tt.path)
		check(t, "ok", ok, tt.wantOK)
		check(t, "text", it.Text, tt.want)
	}
}

// fakePost queues the functions instead of running them, like the GTK main
// loop does until the current callback returns.
type fakePost struct {
	mu     sync.Mutex
	queued []func()
}

func (p *fakePost) post(f func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queued = append(p.queued, f)
}

func (p *fakePost) run() int {
	p.mu.Lock()
	queued := p.queued
	p.queued = nil
	p.mu.Unlock()
	for _, f := range queued {
		f()
	}
	return len(queued)
}

func TestRefreshCoalesces(t *testing.T) {
	var p fakePost
	// Counting the reads of the source counts the renders; without a D-Bus
	// connection, the systray calls do nothing
	renders := 0
	source := func() ([]manager.Entry, string) {
		renders++
		return nil, ""
	}
	tr := New(locale.New(), source, p.post, nil)

	// A burst of changes from many goroutines, before the UI thread runs
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(tr.Refresh)
	}
	wg.Wait()

	check(t, "posted", p.run(), 1)
	check(t, "renders", renders, 1)

	// Once rendered, the next change schedules a new render
	tr.Refresh()
	check(t, "posted after render", p.run(), 1)
	check(t, "renders", renders, 2)
}

// Without a D-Bus connection, fyne.io/systray keeps the menu in memory only:
// enough to check the rendering and the dispatch of the clicks.
func TestClickReadsTheCurrentItem(t *testing.T) {
	var p fakePost
	state := forward.Stopped
	source := func() ([]manager.Entry, string) {
		e := manager.Entry{
			Forward: config.Forward{UUID: "a", Name: "a", LocalPort: 3000, URL: "http://localhost:3000"},
			Status:  forward.Status{State: state},
		}
		return []manager.Entry{e}, ""
	}
	clicked := make(chan Item, 1)
	tr := New(locale.New(), source, p.post, func(it Item) { clicked <- it })

	tr.Refresh()
	p.run()
	first := tr.shown[0]

	// Same shape once active: the switch is updated in place, not recreated
	state = forward.Active
	tr.Refresh()
	p.run()
	if tr.shown[0] != first {
		t.Fatal("the switch was recreated instead of updated in place")
	}
	check(t, "checked", first.Checked(), true)

	first.ClickedCh <- struct{}{}
	// The click goroutine posts the handling to the UI thread
	for p.run() == 0 {
		runtime.Gosched()
	}
	it := <-clicked
	check(t, "action", it.Action, Action{Op: ToggleForward, UUID: "a"})
	// Checked comes from the latest render: a click now means stop
	check(t, "checked at click time", it.Checked, true)
}

func TestIconDrawnOncePerState(t *testing.T) {
	var p fakePost
	summary := Summary{}
	source := func() ([]manager.Entry, string) {
		entries := make([]manager.Entry, 0, summary.Active)
		for range summary.Active {
			entries = append(entries, manager.Entry{Status: forward.Status{State: forward.Active}})
		}
		return entries, ""
	}
	tr := New(locale.New(), source, p.post, nil)
	refresh := func(active int) *Summary {
		summary = Summary{Active: active, Busy: active > 0}
		tr.Refresh()
		p.run()
		return tr.shownIcon
	}

	first := refresh(0)
	// Same state: the icon is neither drawn nor sent again
	if refresh(0) != first {
		t.Error("the same icon was sent again")
	}
	refresh(1)
	refresh(2)
	refresh(1)
	refresh(0)

	check(t, "icons drawn", len(tr.icons), 3)
	check(t, "icon shown", *tr.shownIcon, Summary{})
}
