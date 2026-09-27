package tray

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/manager"
)

func entry(uuid, group string, state forward.State) manager.Entry {
	return manager.Entry{
		Forward: config.Forward{UUID: uuid, Name: uuid, Group: group, Target: "svc/" + uuid, LocalPort: 3000},
		Status:  forward.Status{State: state},
	}
}

// outline summarizes the forwards part of a menu, one token per item:
// "[group]" for a header, "|" for a separator, the UUID for a switch.
func outline(items []Item) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		switch it.Kind {
		case Label:
			parts = append(parts, "["+it.Text+"]")
		case Separator:
			parts = append(parts, "|")
		case Toggle:
			parts = append(parts, it.Action.UUID)
		case Button, Submenu:
		}
	}
	return strings.Join(parts, " ")
}

func TestForwardItemsGrouping(t *testing.T) {
	tests := []struct {
		name    string
		entries []manager.Entry
		want    string
	}{
		{"no group", []manager.Entry{
			entry("a", "", forward.Stopped), entry("b", "", forward.Stopped),
		}, "a b"},
		{"groups", []manager.Entry{
			entry("a", "db", forward.Stopped), entry("b", "db", forward.Stopped), entry("c", "web", forward.Stopped),
		}, "[db] a b | [web] c"},
		{"ungrouped among groups", []manager.Entry{
			entry("a", "db", forward.Stopped), entry("b", "", forward.Stopped),
		}, "[db] a | [" + ungrouped + "] b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check(t, "outline", outline(forwardItems(tt.entries)), tt.want)
		})
	}
}

func TestToggleItem(t *testing.T) {
	boom := errors.New("no ready pod")
	tests := []struct {
		state       forward.State
		err         error
		wantSuffix  string
		wantChecked bool
		wantTooltip string
	}{
		{forward.Stopped, nil, ":3000", false, "svc/a → localhost:3000"},
		{forward.Connecting, boom, "connexion...", true, "no ready pod"},
		{forward.Active, nil, ":3000", true, "svc/a → localhost:3000"},
		// Given up: no longer wanted, so unchecked; a click starts it again
		{forward.Failed, boom, "échec", false, "no ready pod"},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			e := entry("a", "", tt.state)
			e.Status.Err = tt.err
			it := toggleItem(e)

			if !strings.HasSuffix(it.Text, tt.wantSuffix) {
				t.Errorf("text %q should end with %q", it.Text, tt.wantSuffix)
			}
			check(t, "checked", it.Checked, tt.wantChecked)
			check(t, "tooltip", it.Tooltip, tt.wantTooltip)
			check(t, "action", it.Action, Action{Op: ToggleForward, UUID: "a"})
			check(t, "disabled", it.Disabled, false)
		})
	}
}

// find returns the first item of the menu with the given operation.
func find(t *testing.T, items []Item, op Op) Item {
	t.Helper()
	for _, it := range items {
		if it.Action.Op == op {
			return it
		}
	}
	t.Fatalf("no item with operation %d", op)
	return Item{}
}

func TestStopAllEnabled(t *testing.T) {
	tests := []struct {
		state        forward.State
		wantDisabled bool
	}{
		{forward.Stopped, true},
		{forward.Failed, true},
		{forward.Connecting, false},
		{forward.Active, false},
	}
	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			items := Build([]manager.Entry{entry("a", "", forward.Stopped), entry("b", "", tt.state)}, "")
			check(t, "disabled", find(t, items, StopAll).Disabled, tt.wantDisabled)
		})
	}
}

func TestURLSubmenu(t *testing.T) {
	withURL := func(e manager.Entry) manager.Entry {
		e.Forward.URL = "http://localhost:3000/" + e.Forward.UUID
		return e
	}
	items := Build([]manager.Entry{
		withURL(entry("active", "", forward.Active)),
		entry("nourl", "", forward.Active),
		withURL(entry("connecting", "", forward.Connecting)),
	}, "")

	var submenu *Item
	for i := range items {
		if items[i].Kind == Submenu {
			submenu = &items[i]
		}
	}
	if submenu == nil {
		t.Fatal("no submenu")
	}
	want := []Item{
		{
			Kind: Button, Text: "active", Tooltip: "http://localhost:3000/active",
			Action: Action{Op: OpenURL, URL: "http://localhost:3000/active"},
		},
		// Opening it before the forward is ready would show an error page
		{
			Kind: Button, Text: "connecting", Tooltip: "http://localhost:3000/connecting", Disabled: true,
			Action: Action{Op: OpenURL, URL: "http://localhost:3000/connecting"},
		},
	}
	if !reflect.DeepEqual(submenu.Children, want) {
		t.Errorf("submenu children:\n got %+v\nwant %+v", submenu.Children, want)
	}

	// Without any URL, no submenu at all
	for _, it := range Build([]manager.Entry{entry("nourl", "", forward.Active)}, "") {
		if it.Kind == Submenu {
			t.Error("unexpected submenu without URL")
		}
	}
}

func TestBuildHeader(t *testing.T) {
	tests := []struct {
		name    string
		entries []manager.Entry
		problem string
		want    []Item
	}{
		{"problem", []manager.Entry{entry("a", "", forward.Stopped)}, "invalid config", []Item{
			{Kind: Label, Text: "⚠ invalid config", Disabled: true},
			{Kind: Separator},
		}},
		{"empty", nil, "", []Item{
			{Kind: Label, Text: "Aucun forward configuré", Disabled: true},
			{Kind: Separator},
		}},
		// The problem explains the empty list: no second message
		{"empty with problem", nil, "invalid config", []Item{
			{Kind: Label, Text: "⚠ invalid config", Disabled: true},
			{Kind: Separator},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := Build(tt.entries, tt.problem)
			if got := items[:len(tt.want)]; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("menu starts with %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildFooter(t *testing.T) {
	items := Build([]manager.Entry{entry("a", "", forward.Stopped)}, "")

	ops := make([]string, 0, len(items))
	for _, it := range items {
		ops = append(ops, fmt.Sprintf("%d:%d", it.Kind, it.Action.Op))
	}
	got := strings.Join(ops, " ")
	want := fmt.Sprintf("%d:%d %d:%d %d:%d %d:%d %d:%d %d:%d %d:%d",
		Toggle, ToggleForward, Separator, None, Button, StopAll, Separator, None,
		Button, EditConfig, Separator, None, Button, Quit)
	check(t, "kinds and operations", got, want)
}

func TestSummarize(t *testing.T) {
	tests := []struct {
		name  string
		state []forward.State
		want  Summary
	}{
		{"nothing", nil, Summary{}},
		{"stopped and failed", []forward.State{forward.Stopped, forward.Failed}, Summary{}},
		{"connecting only", []forward.State{forward.Connecting}, Summary{Busy: true}},
		{
			"mixed",
			[]forward.State{forward.Active, forward.Connecting, forward.Active, forward.Failed},
			Summary{Active: 2, Busy: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]manager.Entry, 0, len(tt.state))
			for i, s := range tt.state {
				entries = append(entries, entry(fmt.Sprint(i), "", s))
			}
			check(t, "summary", Summarize(entries), tt.want)
		})
	}
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

// A group split in the file shows once: the manager sorts the entries.
func TestSplitGroupShownOnce(t *testing.T) {
	// No forward is started: the factory is never called
	m := manager.New(nil, forward.DefaultPolicy, func() {})
	m.Load([]config.Forward{
		{UUID: "a", Name: "a", Group: "db"},
		{UUID: "b", Name: "b", Group: "web"},
		{UUID: "c", Name: "c", Group: "db"},
	})

	check(t, "outline", outline(forwardItems(m.Snapshot())), "[db] a c | [web] b")
}
