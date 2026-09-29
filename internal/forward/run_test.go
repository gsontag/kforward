package forward

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gsontag/kforward/internal/kube"
)

// step scripts one connection attempt of the fake connector.
type step struct {
	resolveErr error
	ready      bool
	// runFor is how long Forward lasts before returning forwardErr; 0 means
	// until the context is cancelled.
	runFor     time.Duration
	forwardErr error
}

// fakeConnector plays its steps in order, repeating the last one forever.
type fakeConnector struct {
	mu      sync.Mutex
	steps   []step
	current step
	calls   int
}

func (c *fakeConnector) Resolve(context.Context) (kube.Endpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = c.steps[min(c.calls, len(c.steps)-1)]
	c.calls++
	if c.current.resolveErr != nil {
		return kube.Endpoint{}, c.current.resolveErr
	}
	return kube.Endpoint{Pod: fmt.Sprintf("pod-%d", c.calls)}, nil
}

func (c *fakeConnector) Forward(ctx context.Context, _ kube.Endpoint, onReady func()) error {
	c.mu.Lock()
	s := c.current
	c.mu.Unlock()

	if s.ready {
		onReady()
	}
	if s.runFor == 0 {
		<-ctx.Done()
		return nil
	}
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(s.runFor):
		return s.forwardErr
	}
}

// recorder collects the reported statuses; Run may report from its own
// goroutine while the test reads them.
type recorder struct {
	mu       sync.Mutex
	statuses []Status
}

func (r *recorder) report(s Status) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses = append(r.statuses, s)
}

// trace summarizes the statuses as "state@failures", the easiest form to
// compare in a test.
func (r *recorder) trace() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := make([]string, 0, len(r.statuses))
	for _, s := range r.statuses {
		parts = append(parts, fmt.Sprintf("%s@%d", s.State, s.Failures))
	}
	return strings.Join(parts, " ")
}

func (r *recorder) at(i int) Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statuses[i]
}

func (r *recorder) last() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statuses[len(r.statuses)-1]
}

var errLost = errors.New("lost connection")

func TestRunGivesUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &fakeConnector{steps: []step{{resolveErr: errors.New("no ready pod")}}}
		var r recorder
		start := time.Now()

		Run(t.Context(), c, DefaultPolicy, r.report)

		check(t, "trace", r.trace(), "connecting@0 connecting@1 connecting@2 connecting@3 failed@4")
		// Three pauses of RetryDelay, exactly: the clock of the bubble is fake
		check(t, "elapsed", time.Since(start), 3*DefaultPolicy.RetryDelay)
		check(t, "attempts", c.calls, 4)
		if err := r.last().Err; err == nil || err.Error() != "no ready pod" {
			t.Errorf("final error = %v, want the resolution error", err)
		}
	})
}

func TestRunPermanentError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &fakeConnector{steps: []step{{resolveErr: Permanent(errors.New("service not found"))}}}
		var r recorder
		start := time.Now()

		Run(t.Context(), c, DefaultPolicy, r.report)

		check(t, "trace", r.trace(), "connecting@0 failed@1")
		check(t, "elapsed", time.Since(start), 0)
	})
}

func TestRunReconnects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &fakeConnector{steps: []step{
			{ready: true, runFor: time.Second, forwardErr: errLost},
			{ready: true},
		}}
		var r recorder
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go Run(ctx, c, DefaultPolicy, r.report)
		time.Sleep(time.Minute)
		synctest.Wait()

		check(t, "trace", r.trace(), "connecting@0 active@0 connecting@1 active@0")
		// The reconnection reports why it happened
		if err := r.at(2).Err; !errors.Is(err, errLost) {
			t.Errorf("reconnecting status error = %v, want %v", err, errLost)
		}
		check(t, "new endpoint", r.last().Endpoint.Pod, "pod-2")
	})
}

func TestRunStableResetsFailures(t *testing.T) {
	tests := []struct {
		name       string
		activeFor  time.Duration
		wantTrace  string
		wantFailed bool
	}{
		// Each drop after a stable period is a first failure again: never gives up
		{"stable", DefaultPolicy.StableAfter, strings.Repeat("connecting@1 active@0 ", 5), false},
		// Drops before StableAfter pile up until the give-up
		{
			"unstable",
			DefaultPolicy.StableAfter - time.Second,
			"connecting@1 active@0 connecting@2 active@0 " +
				"connecting@3 active@0 failed@4",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := &fakeConnector{
					steps: []step{{ready: true, runFor: tt.activeFor, forwardErr: errLost}},
				}
				var r recorder
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				go Run(ctx, c, DefaultPolicy, r.report)
				// Long enough for five cycles of the stable case
				time.Sleep(5*(tt.activeFor+DefaultPolicy.RetryDelay) + time.Second)
				synctest.Wait()

				got := strings.TrimPrefix(r.trace(), "connecting@0 active@0 ")
				if !strings.HasPrefix(got+" ", tt.wantTrace) {
					t.Errorf("trace = %q, want it to start with %q", got, tt.wantTrace)
				}
				check(t, "failed", r.last().State == Failed, tt.wantFailed)
			})
		})
	}
}

func TestRunCancelWhileActive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &fakeConnector{steps: []step{{ready: true}}}
		var r recorder
		ctx, cancel := context.WithCancel(t.Context())

		returned := make(chan struct{})
		go func() {
			Run(ctx, c, DefaultPolicy, r.report)
			close(returned)
		}()
		synctest.Wait()
		cancel()
		synctest.Wait()

		select {
		case <-returned:
		default:
			t.Fatal("Run did not return after cancellation")
		}
		// A requested stop is not a failure
		check(t, "trace", r.trace(), "connecting@0 active@0 stopped@0")
	})
}

func TestRunCancelWhileWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &fakeConnector{steps: []step{{resolveErr: errors.New("no ready pod")}}}
		var r recorder
		ctx, cancel := context.WithCancel(t.Context())

		go Run(ctx, c, DefaultPolicy, r.report)
		time.Sleep(DefaultPolicy.RetryDelay / 2)
		start := time.Now()
		cancel()
		synctest.Wait()

		check(t, "trace", r.trace(), "connecting@0 stopped@0")
		check(t, "attempts", c.calls, 1)
		// The pause is interrupted: no time has to pass for the stop
		check(t, "elapsed", time.Since(start), 0)
	})
}

func TestRunUnexpectedEnd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &fakeConnector{steps: []step{
			{ready: true, runFor: time.Second},
			{ready: true},
		}}
		var r recorder
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go Run(ctx, c, DefaultPolicy, r.report)
		time.Sleep(time.Minute)
		synctest.Wait()

		check(t, "trace", r.trace(), "connecting@0 active@0 connecting@1 active@0")
		if err := r.at(2).Err; !errors.Is(err, errUnexpectedEnd) {
			t.Errorf("reconnecting status error = %v, want %v", err, errUnexpectedEnd)
		}
	})
}

func TestIsPermanent(t *testing.T) {
	cause := errors.New("forbidden")
	wrapped := fmt.Errorf("svc/grafana: %w", Permanent(cause))

	check(t, "IsPermanent(wrapped)", IsPermanent(wrapped), true)
	check(t, "errors.Is(cause)", errors.Is(wrapped, cause), true)
	check(t, "message", wrapped.Error(), "svc/grafana: forbidden")
	check(t, "IsPermanent(cause)", IsPermanent(cause), false)
	check(t, "IsPermanent(nil)", IsPermanent(nil), false)
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{Stopped, "stopped"},
		{Connecting, "connecting"},
		{Active, "active"},
		{Failed, "failed"},
		{State(42), "unknown"},
	}
	for _, tt := range tests {
		check(t, fmt.Sprintf("State(%d)", int(tt.state)), tt.state.String(), tt.want)
	}
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
