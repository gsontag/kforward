// Package notify tells the user, through desktop notifications, what went
// wrong while no window was looking: a forward that gave up, a configuration
// that became invalid.
package notify

import (
	"github.com/gsontag/kforward/internal/forward"
	"github.com/gsontag/kforward/internal/locale"
	"github.com/gsontag/kforward/internal/manager"
)

// Notification is a message for the desktop.
type Notification struct {
	// Key identifies what it is about: a newer notification with the same
	// key replaces the previous one instead of piling up.
	Key           string
	Summary, Body string
}

// configKey is the key of the notifications about the configuration.
const configKey = "config"

// Tracker turns the successive states of the application into
// notifications: only what the user did not ask for deserves one.
type Tracker struct {
	tr      *locale.Translator
	states  map[string]forward.State
	problem string
}

// NewTracker returns a tracker that has seen nothing yet.
func NewTracker(tr *locale.Translator) *Tracker {
	return &Tracker{tr: tr, states: map[string]forward.State{}}
}

// Update returns the notifications due since the previous update: forwards
// that gave up, and a configuration problem that appeared or changed.
func (t *Tracker) Update(entries []manager.Entry, problem string) []Notification {
	var due []Notification
	states := make(map[string]forward.State, len(entries))
	for _, e := range entries {
		id, s := e.Forward.UUID, e.Status
		states[id] = s.State
		// A failed forward stays failed: notify the change, not the state
		if s.State == forward.Failed && t.states[id] != forward.Failed {
			body := ""
			if s.Err != nil {
				body = s.Err.Error()
			}
			due = append(due, Notification{
				Key:     id,
				Summary: t.tr.T(msgFailed, map[string]any{"Name": e.Forward.Name}),
				Body:    body,
			})
		}
	}
	t.states = states

	if problem != "" && problem != t.problem {
		due = append(
			due,
			Notification{Key: configKey, Summary: t.tr.T(msgProblem, nil), Body: problem},
		)
	}
	t.problem = problem
	return due
}
