// Package window shows the forwards and their states in a GTK window.
package window

import (
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/locale"
	"gsontag.fr/kforward/internal/manager"
)

// Row is what the window shows of one forward: plain data, like tray.Item.
type Row struct {
	UUID string
	// Group is the header of the row, empty when the configuration has none.
	Group string
	Title string
	// Subtitle is where the forward goes; Status its state in words.
	Subtitle, Status string
	// Error is the reason of the last failure, empty when there is none.
	Error string
	// Wanted is the position of the switch, Active its "on" state: they
	// differ while connecting, as in a GtkSwitch.
	Wanted, Active bool
}

// Rows returns the rows showing entries, in the language of tr; entries come
// sorted by the manager, so each group is contiguous.
func Rows(entries []manager.Entry, tr *locale.Translator) []Row {
	grouped := false
	for _, e := range entries {
		grouped = grouped || e.Forward.Group != ""
	}

	rows := make([]Row, 0, len(entries))
	for _, e := range entries {
		f, s := e.Forward, e.Status
		// Without any group in the configuration, no header at all
		group := f.Group
		if grouped && group == "" {
			group = tr.T(msgUngrouped, nil)
		}
		row := Row{
			UUID:  f.UUID,
			Group: group,
			Title: f.Name,
			Subtitle: tr.T(msgTarget, map[string]any{
				"Context": orDefault(
					f.Context,
				), "Namespace": orDefault(f.Namespace), "Target": f.Target,
			}),
			Status: status(s, f.BindAddress(), f.LocalPort, tr),
			Wanted: s.State == forward.Connecting || s.State == forward.Active,
			Active: s.State == forward.Active,
		}
		if s.Err != nil && s.State != forward.Active {
			// Technical and searchable: errors stay in English
			row.Error = s.Err.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func status(s forward.Status, address string, port uint16, tr *locale.Translator) string {
	switch s.State {
	case forward.Connecting:
		if s.Failures > 0 {
			return tr.N(msgRetrying, s.Failures, nil)
		}
		return tr.T(msgConnecting, nil)
	case forward.Active:
		return tr.T(msgActive, map[string]any{"Address": address, "Port": port})
	case forward.Failed:
		return tr.T(msgFailed, nil)
	case forward.Stopped:
	}
	return tr.T(msgStopped, nil)
}

// orDefault shows an empty context or namespace as the kubeconfig default.
func orDefault(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
