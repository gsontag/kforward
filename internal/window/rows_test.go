package window

import (
	"errors"
	"strings"
	"testing"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/locale"
	"gsontag.fr/kforward/internal/manager"
)

// English, whatever the language of the machine running the tests
var tr = locale.New()

func entry(name, group string, s forward.Status) manager.Entry {
	return manager.Entry{
		Forward: config.Forward{
			UUID: name, Name: name, Group: group,
			Context: "prod", Namespace: "monitoring", Target: "svc/" + name, LocalPort: 3000,
		},
		Status: s,
	}
}

func row(t *testing.T, e manager.Entry) Row {
	t.Helper()
	rows := Rows([]manager.Entry{e}, tr)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	return rows[0]
}

func TestRowStates(t *testing.T) {
	lost := errors.New("lost connection to pod")
	tests := []struct {
		name       string
		status     forward.Status
		wantStatus string
		wantWanted bool
		wantActive bool
		wantError  string
	}{
		{"stopped", forward.Status{State: forward.Stopped}, "Stopped", false, false, ""},
		{"connecting", forward.Status{State: forward.Connecting}, "Connecting…", true, false, ""},
		// Switch moved but not on yet: the GtkSwitch delayed state
		{
			"reconnecting once",
			forward.Status{State: forward.Connecting, Failures: 1, Err: lost},
			"Reconnecting, 1 failure", true, false, "lost connection to pod",
		},
		{
			"reconnecting twice",
			forward.Status{State: forward.Connecting, Failures: 2, Err: lost},
			"Reconnecting, 2 failures", true, false, "lost connection to pod",
		},
		{
			"active",
			forward.Status{State: forward.Active},
			"Active on 127.0.0.1:3000",
			true,
			true,
			"",
		},
		{
			"failed",
			forward.Status{State: forward.Failed, Failures: 4, Err: lost},
			"Failed", false, false, "lost connection to pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := row(t, entry("grafana", "", tt.status))
			check(t, "status", r.Status, tt.wantStatus)
			check(t, "wanted", r.Wanted, tt.wantWanted)
			check(t, "active", r.Active, tt.wantActive)
			check(t, "error", r.Error, tt.wantError)
		})
	}
}

func TestRowErrorHiddenOnceActive(t *testing.T) {
	// The error of the last reconnection is kept in the status, but the
	// forward works again: nothing to worry the user about
	r := row(t, entry("grafana", "", forward.Status{State: forward.Active, Err: errors.New("old")}))
	check(t, "error", r.Error, "")
}

func TestRowTexts(t *testing.T) {
	r := row(t, entry("grafana", "", forward.Status{}))
	check(t, "uuid", r.UUID, "grafana")
	check(t, "title", r.Title, "grafana")
	check(t, "subtitle", r.Subtitle, "prod · monitoring/svc/grafana")

	// Empty context and namespace mean the defaults of the kubeconfig
	e := entry("grafana", "", forward.Status{})
	e.Forward.Context, e.Forward.Namespace = "", ""
	check(t, "subtitle with defaults", row(t, e).Subtitle, "- · -/svc/grafana")
}

func TestRowBindAddress(t *testing.T) {
	e := entry("keycloak", "", forward.Status{State: forward.Active})
	e.Forward.Address = "0.0.0.0"
	// Exposed to the network: the window says so
	check(t, "status", row(t, e).Status, "Active on 0.0.0.0:3000")
}

func TestRowGroups(t *testing.T) {
	groups := func(entries ...manager.Entry) string {
		rows := Rows(entries, tr)
		parts := make([]string, 0, len(rows))
		for _, r := range rows {
			parts = append(parts, r.Title+"="+r.Group)
		}
		return strings.Join(parts, " ")
	}
	stopped := forward.Status{}

	check(t, "no group at all", groups(entry("a", "", stopped), entry("b", "", stopped)), "a= b=")
	check(
		t,
		"some groups",
		groups(entry("a", "db", stopped), entry("b", "", stopped)),
		"a=db b=Other",
	)
	check(t, "empty", groups(), "")
}

func TestRowsInFrench(t *testing.T) {
	fr := locale.New("fr")
	rows := Rows([]manager.Entry{
		entry("a", "db", forward.Status{State: forward.Active}),
		entry("b", "", forward.Status{State: forward.Connecting, Failures: 2}),
		entry("c", "", forward.Status{State: forward.Failed}),
	}, fr)

	got := make([]string, 0, len(rows))
	for _, r := range rows {
		got = append(got, r.Group+": "+r.Status)
	}
	want := "db: Actif sur 127.0.0.1:3000 | Autres: Reconnexion, 2 échecs | Autres: Échec"
	check(t, "rows", strings.Join(got, " | "), want)
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
