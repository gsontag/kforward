package notify

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/locale"
	"gsontag.fr/kforward/internal/manager"
)

// English, whatever the language of the machine running the tests
var tr = locale.New()

func entry(name string, state forward.State, err error) manager.Entry {
	return manager.Entry{
		Forward: config.Forward{UUID: name, Name: name},
		Status:  forward.Status{State: state, Err: err},
	}
}

// summaries lists the notifications as "key: summary (body)".
func summaries(ns []Notification) string {
	parts := make([]string, 0, len(ns))
	for _, n := range ns {
		parts = append(parts, fmt.Sprintf("%s: %s (%s)", n.Key, n.Summary, n.Body))
	}
	return strings.Join(parts, " | ")
}

var errNotFound = errors.New(`services "absent" not found`)

func TestFailureNotifiedOnce(t *testing.T) {
	tk := NewTracker(tr)

	check(t, "connecting", summaries(tk.Update([]manager.Entry{entry("grafana", forward.Connecting, nil)}, "")), "")
	check(t, "failed",
		summaries(tk.Update([]manager.Entry{entry("grafana", forward.Failed, errNotFound)}, "")),
		`grafana: grafana: forward stopped (services "absent" not found)`)

	// Still failed at the next updates: nothing new to say
	for range 3 {
		check(
			t,
			"still failed",
			summaries(tk.Update([]manager.Entry{entry("grafana", forward.Failed, errNotFound)}, "")),
			"",
		)
	}
}

func TestFailureAgainAfterRestart(t *testing.T) {
	tk := NewTracker(tr)
	failed := []manager.Entry{entry("grafana", forward.Failed, errNotFound)}
	tk.Update(failed, "")

	// The user starts it again, and it fails again: a new failure
	tk.Update([]manager.Entry{entry("grafana", forward.Connecting, nil)}, "")
	if got := tk.Update(failed, ""); len(got) != 1 {
		t.Errorf("got %d notifications, want 1", len(got))
	}
}

func TestOtherStatesNotNotified(t *testing.T) {
	tk := NewTracker(tr)
	for _, state := range []forward.State{forward.Connecting, forward.Active, forward.Stopped, forward.Connecting} {
		// Reconnecting with an error is not worth an interruption either
		got := tk.Update([]manager.Entry{entry("grafana", state, errors.New("lost connection to pod"))}, "")
		check(t, state.String(), summaries(got), "")
	}
}

// A forward that autostarts and fails is failed at the first update.
func TestFailureAtFirstUpdate(t *testing.T) {
	got := NewTracker(tr).Update([]manager.Entry{entry("grafana", forward.Failed, nil)}, "")
	check(t, "notifications", summaries(got), "grafana: grafana: forward stopped ()")
}

func TestRemovedThenAddedBackFailed(t *testing.T) {
	tk := NewTracker(tr)
	failed := []manager.Entry{entry("grafana", forward.Failed, errNotFound)}
	tk.Update(failed, "")

	// Removed from the configuration: forgotten
	tk.Update(nil, "")
	if got := tk.Update(failed, ""); len(got) != 1 {
		t.Errorf("got %d notifications, want 1", len(got))
	}
}

func TestSeveralForwards(t *testing.T) {
	got := NewTracker(tr).Update([]manager.Entry{
		entry("a", forward.Failed, nil),
		entry("b", forward.Active, nil),
		entry("c", forward.Failed, nil),
	}, "")
	check(t, "keys", len(got), 2)
	check(t, "first", got[0].Key, "a")
	check(t, "second", got[1].Key, "c")
}

func TestConfigProblem(t *testing.T) {
	tk := NewTracker(tr)
	problem := func(p string) string { return summaries(tk.Update(nil, p)) }

	check(t, "appears", problem("parse config.json: unexpected end"),
		"config: Configuration problem (parse config.json: unexpected end)")
	check(t, "same", problem("parse config.json: unexpected end"), "")
	check(t, "another one", problem("invalid config.json: name is required"),
		"config: Configuration problem (invalid config.json: name is required)")
	// Fixed: the menu and the window show it, no need to interrupt
	check(t, "fixed", problem(""), "")
	check(t, "appears again", problem("parse config.json: unexpected end"),
		"config: Configuration problem (parse config.json: unexpected end)")
}

func TestNotificationsInFrench(t *testing.T) {
	tk := NewTracker(locale.New("fr"))
	got := tk.Update([]manager.Entry{entry("grafana", forward.Failed, nil)}, "broken")
	check(t, "forward", got[0].Summary, "grafana: forward arrêté")
	check(t, "config", got[1].Summary, "Erreur de configuration")
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
