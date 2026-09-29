package tray

import (
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"

	"fyne.io/systray"

	"gsontag.fr/kforward/internal/icon"
	"gsontag.fr/kforward/internal/locale"
)

// iconSize is the side of the drawn icon: large enough for HiDPI panels,
// which scale it down.
const iconSize = 64

// Source provides what the tray shows.
type Source func() State

// Tray renders the menu in the status notifier icon. Except Refresh, its
// methods run on the UI thread: the one post executes functions on.
type Tray struct {
	tr     *locale.Translator
	source Source
	post   func(func())
	handle func(Item)

	// pending coalesces the refreshes requested before the UI thread runs one
	pending atomic.Bool

	items []Item
	shown []*systray.MenuItem
	shape string

	// icons cache the drawn icons by state; shownIcon avoids sending the
	// same icon again, which would make the panel reload it
	icons     map[Summary][]byte
	shownIcon *Summary
}

// New returns a tray; handle receives the clicked items, on the UI thread.
func New(tr *locale.Translator, source Source, post func(func()), handle func(Item)) *Tray {
	return &Tray{tr: tr, source: source, post: post, handle: handle, icons: map[Summary][]byte{}}
}

// Refresh schedules an update of the menu. It is safe to call from any
// goroutine, as often as needed: pending updates collapse into one.
func (t *Tray) Refresh() {
	if t.pending.Swap(true) {
		return
	}
	t.post(func() {
		t.pending.Store(false)
		t.render()
	})
}

func (t *Tray) render() {
	state := t.source()
	items := Build(state, t.tr)

	summary := Summarize(state.Entries)
	t.showIcon(summary)
	systray.SetTooltip(Tooltip(summary, t.tr))

	// Rebuilding a menu the user has open makes it jump: update in place
	// whenever the structure is unchanged
	if s := shape(items); s != t.shape {
		systray.ResetMenu()
		t.shown = nil
		t.add(items, nil, nil)
		t.shape = s
	} else {
		t.update(items, t.shown)
	}
	t.items = items
}

// shape describes the structure of a menu: two menus with the same shape
// only differ by texts and states, and can be updated in place.
func shape(items []Item) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(strconv.Itoa(int(it.Kind)))
		if it.Kind == Submenu {
			b.WriteString("(")
			b.WriteString(shape(it.Children))
			b.WriteString(")")
		}
		b.WriteByte(',')
	}
	return b.String()
}

// add creates the menu items, depth first; path locates each one in t.items.
func (t *Tray) add(items []Item, parent *systray.MenuItem, path []int) {
	for i, it := range items {
		itemPath := append(path[:len(path):len(path)], i)
		if it.Kind == Separator {
			if parent == nil {
				systray.AddSeparator()
			} else {
				systray.AddSeparator()
			}
			continue
		}

		var mi *systray.MenuItem
		switch {
		case parent == nil && it.Kind == Toggle:
			mi = systray.AddMenuItemCheckbox(it.Text, it.Tooltip, it.Checked)
		case parent == nil:
			mi = systray.AddMenuItem(it.Text, it.Tooltip)
		case it.Kind == Toggle:
			mi = parent.AddSubMenuItemCheckbox(it.Text, it.Tooltip, it.Checked)
		default:
			mi = parent.AddSubMenuItem(it.Text, it.Tooltip)
		}
		if it.Disabled {
			mi.Disabled()
		}
		t.shown = append(t.shown, mi)
		go t.listen(mi, itemPath)

		if it.Kind == Submenu {
			t.add(it.Children, mi, itemPath)
		}
	}
}

// update changes the items in place; shown lists them in the order of add.
func (t *Tray) update(items []Item, shown []*systray.MenuItem) []*systray.MenuItem {
	for _, it := range items {
		if it.Kind == Separator {
			continue
		}
		mi := shown[0]
		shown = shown[1:]
		mi.SetTitle(it.Text)
		mi.SetTooltip(it.Tooltip)
		if it.Checked {
			mi.Check()
		} else {
			mi.Uncheck()
		}
		if it.Disabled {
			mi.Disable()
		} else {
			mi.Enable()
		}
		if it.Kind == Submenu {
			shown = t.update(it.Children, shown)
		}
	}
	return shown
}

// listen forwards the clicks of mi until the item is removed, which closes
// its channel.
func (t *Tray) listen(mi *systray.MenuItem, path []int) {
	for range mi.ClickedCh {
		t.post(func() {
			// Read at click time: the item may have been updated in place
			if it, ok := at(t.items, path); ok {
				t.handle(it)
			}
		})
	}
}

func (t *Tray) showIcon(s Summary) {
	if t.shownIcon != nil && *t.shownIcon == s {
		return
	}
	png, ok := t.icons[s]
	if !ok {
		var err error
		if png, err = icon.Render(iconSize, s.Active, s.Busy); err != nil {
			slog.Warn("tray icon", "err", err)
			return
		}
		t.icons[s] = png
	}
	systray.SetIcon(png)
	t.shownIcon = &s
}

func at(items []Item, path []int) (Item, bool) {
	var it Item
	for _, i := range path {
		if i >= len(items) {
			return Item{}, false
		}
		it = items[i]
		items = it.Children
	}
	return it, len(path) > 0
}
