package gtk

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// iterateUntil runs the main loop until done, or fails after a second.
func iterateUntil(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		iterate()
		time.Sleep(time.Millisecond)
	}
}

func TestIdleAddRunsOnce(t *testing.T) {
	var calls atomic.Int32
	IdleAdd(func() { calls.Add(1) })

	iterateUntil(t, func() bool { return calls.Load() > 0 })
	// Nothing left to run: an idle function does not repeat
	iterate()
	if got := calls.Load(); got != 1 {
		t.Errorf("called %d times, want 1", got)
	}
}

func TestIdleAddFromGoroutines(t *testing.T) {
	const n = 100
	// Only the main loop increments it: no lock needed, the race detector checks
	calls := 0
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { IdleAdd(func() { calls++ }) })
	}
	wg.Wait()

	iterateUntil(t, func() bool { return calls == n })
}

func TestTimeoutRunsOnceAfterTheDelay(t *testing.T) {
	calls := 0
	start := time.Now()
	TimeoutAdd(20, func() { calls++ })

	iterate()
	if calls != 0 {
		t.Fatal("called before the delay")
	}
	iterateUntil(t, func() bool { return calls > 0 })
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("called after %s, want 20ms at least", elapsed)
	}
	time.Sleep(30 * time.Millisecond)
	iterate()
	if calls != 1 {
		t.Errorf("called %d times, want 1", calls)
	}
}

func TestSourceRemove(t *testing.T) {
	called := false
	h := TimeoutAdd(1, func() { called = true })
	SourceRemove(h)

	time.Sleep(10 * time.Millisecond)
	iterate()
	if called {
		t.Error("called after SourceRemove")
	}
}
