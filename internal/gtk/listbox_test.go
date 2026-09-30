package gtk

import (
	"fmt"
	"strings"
	"testing"
)

// newList returns a list of labels named after texts, with the header
// function the window uses: a header above the first row of each group.
func newList(texts ...string) (*ListBox, *[]string) {
	calls := &[]string{}
	l := NewListBox()
	l.SetHeaderFunc(func(row, before *ListBoxRow) {
		b := -1
		if before != nil {
			b = before.Index()
		}
		*calls = append(*calls, fmt.Sprintf("%d<%d", row.Index(), b))
		group := func(r *ListBoxRow) string { return texts[r.Index()][:1] }
		if before != nil && group(before) == group(row) {
			row.SetHeader(nil)
			return
		}
		row.SetHeader(NewLabel(group(row)))
	})
	for _, text := range texts {
		l.Append(NewLabel(text))
	}
	return l, calls
}

func TestListBoxRows(t *testing.T) {
	var indexes []int
	var outside *ListBoxRow
	onMain(t, func() {
		l, _ := newList("a1", "a2", "b1")
		defer l.obj.release()
		for i := range 3 {
			indexes = append(indexes, l.RowAtIndex(i).Index())
		}
		outside = l.RowAtIndex(3)
	})
	check(t, "indexes", fmt.Sprint(indexes), "[0 1 2]")
	if outside != nil {
		t.Error("RowAtIndex past the end: got a row, want nil")
	}
}

func TestListBoxHeaders(t *testing.T) {
	var headers []bool
	var calls []string
	onMain(t, func() {
		l, got := newList("a1", "a2", "b1")
		defer l.obj.release()
		for i := range 3 {
			headers = append(headers, l.RowAtIndex(i).HasHeader())
		}
		*got = nil
		l.InvalidateHeaders()
		calls = *got
	})
	// The first row of each group only
	check(t, "headers", fmt.Sprint(headers), "[true false true]")
	// Each row is given the one before it, none for the first
	check(t, "calls after invalidate", strings.Join(calls, " "), "0<-1 1<0 2<1")
}

func TestListBoxRowActivated(t *testing.T) {
	var activated []int
	onMain(t, func() {
		l, _ := newList("a1", "a2", "b1")
		defer l.obj.release()
		l.ConnectRowActivated(func(row *ListBoxRow) { activated = append(activated, row.Index()) })
		// What the list emits for a click, or Enter, on a row
		l.obj.emitWith("row-activated", l.RowAtIndex(2).obj)
		l.obj.emitWith("row-activated", l.RowAtIndex(0).obj)
	})
	check(t, "activated", fmt.Sprint(activated), "[2 0]")
}

func TestListBoxRemoveAll(t *testing.T) {
	before := live.Load()
	var rowAlive, rowAfter, headerFuncKept bool
	var rows int
	onMain(t, func() {
		l, _ := newList("a1", "b1")
		row := l.RowAtIndex(0)
		rowAlive = row.obj.alive()

		l.RemoveAll()
		rowAfter = row.obj.alive()
		if l.RowAtIndex(0) == nil {
			rows = 0
		}
		// The header function stays for the rows to come
		headerFuncKept = live.Load() > before

		l.obj.release()
	})
	check(t, "row alive in its list", rowAlive, true)
	check(t, "row alive after RemoveAll", rowAfter, false)
	check(t, "rows left", rows, 0)
	check(t, "header function kept", headerFuncKept, true)
	// Freed with the list: its header function, and every handler
	check(t, "handles not released", live.Load()-before, int64(0))
}

func TestScrolledWindowOwnsItsChild(t *testing.T) {
	var alive, after bool
	onMain(t, func() {
		s := NewScrolledWindow()
		s.SetPropagateNaturalWidth(true)
		s.SetPropagateNaturalHeight(true)
		s.SetMaxContentHeight(300)
		l, _ := newList("a1")
		s.SetChild(l)
		alive = l.obj.alive()
		s.obj.release()
		after = l.obj.alive()
	})
	check(t, "child alive in its window", alive, true)
	check(t, "child alive after its window", after, false)
}
