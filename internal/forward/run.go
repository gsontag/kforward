package forward

import (
	"context"
	"errors"
	"time"

	"gsontag.fr/kforward/internal/kube"
)

var errUnexpectedEnd = errors.New("forward ended unexpectedly")

// Connector opens the connections of one forward. It stands for the cluster,
// so that the retry logic can be tested without one.
type Connector interface {
	// Resolve finds the endpoint to connect to.
	Resolve(ctx context.Context) (kube.Endpoint, error)
	// Forward blocks while the forward runs, with the contract of kube.Forward.
	Forward(ctx context.Context, endpoint kube.Endpoint, onReady func()) error
}

// Policy tunes the reconnections.
type Policy struct {
	// RetryDelay is the pause before a reconnection.
	RetryDelay time.Duration
	// MaxRetries is the number of consecutive failures tolerated.
	MaxRetries int
	// StableAfter is how long a forward must stay active to reset the failures.
	StableAfter time.Duration
}

// DefaultPolicy is the policy of the Python version.
var DefaultPolicy = Policy{
	RetryDelay:  3 * time.Second,
	MaxRetries:  3,
	StableAfter: 30 * time.Second,
}

// Run keeps the forward up until ctx is cancelled, reported as Stopped, or
// until it gives up, reported as Failed. report is always called from the
// goroutine of Run, so it is never called concurrently, and the final status
// is reported before Run returns.
func Run(ctx context.Context, c Connector, p Policy, report func(Status)) {
	failures := 0
	var lastErr error
	for {
		report(Status{State: Connecting, Err: lastErr, Failures: failures})

		readyAt, err := attempt(ctx, c, report)
		if ctx.Err() != nil {
			report(Status{State: Stopped})
			return
		}
		if err == nil {
			err = errUnexpectedEnd
		}

		// Only failures in a row count: a long active period starts afresh
		if !readyAt.IsZero() && time.Since(readyAt) >= p.StableAfter {
			failures = 0
		}
		failures++
		lastErr = err

		if IsPermanent(err) || failures > p.MaxRetries {
			report(Status{State: Failed, Err: err, Failures: failures})
			return
		}

		timer := time.NewTimer(p.RetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			report(Status{State: Stopped})
			return
		case <-timer.C:
		}
	}
}

// attempt runs one connection. It returns when it ended, and the time it
// became active, zero if it never did.
func attempt(ctx context.Context, c Connector, report func(Status)) (time.Time, error) {
	endpoint, err := c.Resolve(ctx)
	if err != nil {
		return time.Time{}, err
	}

	// Buffered: neither sender blocks, even once attempt has returned
	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.Forward(ctx, endpoint, func() { ready <- struct{}{} })
	}()

	// Reporting from here, not from onReady, keeps every report on one goroutine
	var readyAt time.Time
	for {
		select {
		case <-ready:
			readyAt = time.Now()
			report(Status{State: Active, Endpoint: endpoint})
		case err := <-done:
			return readyAt, err
		}
	}
}
