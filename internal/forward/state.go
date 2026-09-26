// Package forward keeps port-forwards up: it reconnects them when they drop
// and gives up after repeated failures.
package forward

import (
	"errors"

	"gsontag.fr/kforward/internal/kube"
)

// State is the lifecycle stage of a forward.
type State int

// Forward states, in lifecycle order.
const (
	Stopped State = iota
	Connecting
	Active
	Failed
)

func (s State) String() string {
	switch s {
	case Stopped:
		return "stopped"
	case Connecting:
		return "connecting"
	case Active:
		return "active"
	case Failed:
		return "failed"
	default:
		return "unknown"
	}
}

// Status is a snapshot of a forward, reported on every change.
type Status struct {
	State State
	// Endpoint is the pod and port in use, set when Active.
	Endpoint kube.Endpoint
	// Err is the last failure: the reason of a reconnection or of the give-up.
	Err error
	// Failures counts the consecutive failed attempts.
	Failures int
}

// permanentError marks an error that retrying cannot fix.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so that Run gives up at once instead of retrying.
func Permanent(err error) error {
	return &permanentError{err: err}
}

// IsPermanent reports whether err, or an error it wraps, was marked Permanent.
func IsPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}
