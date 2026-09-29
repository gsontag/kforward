package editor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gsontag/kforward/internal/kube"
)

// fakeSource answers after a delay, per question; it records the questions
// asked and those whose context was cancelled.
type fakeSource struct {
	mu        sync.Mutex
	delays    map[string]time.Duration
	errs      map[string]error
	asked     []string
	cancelled []string
}

func (s *fakeSource) answer(ctx context.Context, question string) error {
	s.mu.Lock()
	s.asked = append(s.asked, question)
	delay, err := s.delays[question], s.errs[question]
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		s.cancelled = append(s.cancelled, question)
		s.mu.Unlock()
		return ctx.Err()
	case <-time.After(delay):
		return err
	}
}

func (s *fakeSource) Namespaces(ctx context.Context, kubeContext string) ([]string, error) {
	if err := s.answer(ctx, "namespaces of "+kubeContext); err != nil {
		return nil, err
	}
	return []string{kubeContext + "-a", kubeContext + "-b"}, nil
}

func (s *fakeSource) Targets(ctx context.Context, kubeContext, namespace string) ([]string, error) {
	if err := s.answer(ctx, "targets of "+kubeContext+"/"+namespace); err != nil {
		return nil, err
	}
	return []string{"svc/" + namespace}, nil
}

func (s *fakeSource) Ports(
	ctx context.Context,
	kubeContext, namespace, target string,
) ([]kube.Port, error) {
	if err := s.answer(ctx, "ports of "+kubeContext+"/"+namespace+"/"+target); err != nil {
		return nil, err
	}
	return []kube.Port{{Name: "web", Number: 80}, {Number: 22}}, nil
}

// questions lists the questions asked and cancelled, sorted like deliveries.
func (s *fakeSource) questions() (asked, cancelled string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return delivered(s.asked).String(), delivered(s.cancelled).String()
}

// uiThread queues the posted functions and runs them on the test goroutine,
// like the GTK main loop runs them one at a time.
type uiThread struct {
	mu     sync.Mutex
	queued []func()
}

func (u *uiThread) post(f func()) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.queued = append(u.queued, f)
}

// runQueued runs the functions posted so far, once.
func (u *uiThread) runQueued() int {
	u.mu.Lock()
	queued := u.queued
	u.queued = nil
	u.mu.Unlock()
	for _, f := range queued {
		f()
	}
	return len(queued)
}

// drain runs the posted functions until none is left, letting the goroutines
// of the bubble run in between.
func (u *uiThread) drain() {
	for {
		synctest.Wait()
		if u.runQueued() == 0 {
			return
		}
	}
}

// settle runs what is due now, lets d pass, then runs what became due.
func (u *uiThread) settle(d time.Duration) {
	u.drain()
	time.Sleep(d)
	u.drain()
}

// delivered records what the suggester delivers, as "level: values".
type delivered []string

func (d *delivered) deliver(l Level, choices []Choice, err error) {
	values := make([]string, 0, len(choices))
	for _, c := range choices {
		values = append(values, c.Value)
	}
	line := fmt.Sprintf("%s: %s", levelNames[l], strings.Join(values, ","))
	if err != nil {
		line += " (" + err.Error() + ")"
	}
	*d = append(*d, line)
}

var levelNames = map[Level]string{Namespaces: "namespaces", Targets: "targets", Ports: "ports"}

// String lists the deliveries sorted: the levels are queried in parallel, so
// they arrive in any order.
func (d delivered) String() string {
	lines := slices.Clone(d)
	slices.Sort(lines)
	return strings.Join(lines, " | ")
}

func newTestSuggester(src *fakeSource) (*Suggester, *uiThread, *delivered) {
	var ui uiThread
	var got delivered
	return NewSuggester(src, ui.post, got.deliver), &ui, &got
}

func TestContextQueriesEveryLevel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{}
		s, ui, got := newTestSuggester(src)
		defer s.Close()

		s.SetContext("prod")
		ui.settle(0)

		// No target yet: an empty list of ports, without any question
		check(t, "delivered", got.String(), "namespaces: prod-a,prod-b | ports:  | targets: svc/")
		asked, _ := src.questions()
		check(t, "asked", asked, "namespaces of prod | targets of prod/")
	})
}

func TestStaleAnswerIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{delays: map[string]time.Duration{"namespaces of slow": 5 * time.Second}}
		s, ui, got := newTestSuggester(src)
		defer s.Close()

		s.SetContext("slow")
		ui.settle(time.Second)
		*got = nil
		s.SetContext("fast")
		ui.settle(10 * time.Second)

		// Only the namespaces of the second context: the slow answer was cancelled
		check(t, "delivered", got.String(), "namespaces: fast-a,fast-b | ports:  | targets: svc/")
		_, cancelled := src.questions()
		check(t, "cancelled", cancelled, "namespaces of slow")
	})
}

// An answer already posted when the context changes must not be shown: it
// is too late to cancel its query.
func TestAnswerRacingAChangeIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{}
		s, ui, got := newTestSuggester(src)
		defer s.Close()

		s.SetContext("old")
		synctest.Wait()
		ui.runQueued() // the timers: the queries start
		synctest.Wait()
		// The answers are posted, not run yet: the context changes now
		s.SetContext("new")
		ui.settle(0)

		check(t, "delivered", got.String(), "namespaces: new-a,new-b | ports:  | targets: svc/")
	})
}

func TestTypingDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{}
		s, ui, got := newTestSuggester(src)
		defer s.Close()
		s.SetContext("prod")
		ui.settle(0)
		*got = nil
		src.mu.Lock()
		src.asked = nil
		src.mu.Unlock()

		for _, typed := range []string{"m", "mo", "mon"} {
			s.SetNamespace(typed)
			ui.settle(TypingDelay / 4)
		}
		asked, _ := src.questions()
		check(t, "asked while typing", asked, "")

		ui.settle(TypingDelay)
		asked, _ = src.questions()
		// One question, with the final value; the namespaces are not asked again
		check(t, "asked after the pause", asked, "targets of prod/mon")
		check(t, "delivered", got.String(), "ports:  | targets: svc/mon")
	})
}

func TestTargetQueriesPorts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{}
		s, ui, got := newTestSuggester(src)
		defer s.Close()
		s.SetContext("prod")
		s.SetNamespace("monitoring")
		ui.settle(TypingDelay)
		*got = nil

		s.SetTarget("svc/grafana")
		ui.settle(TypingDelay)

		check(t, "delivered", got.String(), "ports: web,22")
	})
}

func TestErrorDelivered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		forbidden := errors.New(`namespaces is forbidden: User "gerard" cannot list resource`)
		src := &fakeSource{errs: map[string]error{"namespaces of prod": forbidden}}
		var ui uiThread
		var gotErr error
		s := NewSuggester(src, ui.post, func(l Level, _ []Choice, err error) {
			if l == Namespaces {
				gotErr = err
			}
		})
		defer s.Close()

		s.SetContext("prod")
		ui.settle(0)

		if !errors.Is(gotErr, forbidden) {
			t.Errorf("got error %v, want %v", gotErr, forbidden)
		}
	})
}

func TestCloseCancelsAndDeliversNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{delays: map[string]time.Duration{"namespaces of prod": time.Minute}}
		s, ui, got := newTestSuggester(src)

		s.SetContext("prod")
		ui.settle(time.Second)
		*got = nil
		s.Close()
		ui.settle(2 * time.Minute)

		check(t, "delivered after close", got.String(), "")
		_, cancelled := src.questions()
		check(t, "cancelled", cancelled, "namespaces of prod")
	})
}

func TestQueryTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{delays: map[string]time.Duration{"namespaces of dead": time.Hour}}
		s, ui, got := newTestSuggester(src)
		defer s.Close()

		start := time.Now()
		s.SetContext("dead")
		ui.settle(QueryTimeout)

		check(t, "waited", time.Since(start), QueryTimeout)
		if len(*got) == 0 ||
			!strings.Contains((*got)[len(*got)-1], context.DeadlineExceeded.Error()) {
			t.Errorf("delivered %v, want the namespaces to fail with a deadline", *got)
		}
	})
}

func TestPortChoices(t *testing.T) {
	got := portChoices([]kube.Port{{Name: "web", Number: 80}, {Number: 22}})
	want := []Choice{
		// The name survives a change of the port number
		{Value: "web", Label: "web · 80", Port: 80},
		{Value: "22", Label: "22", Port: 22},
	}
	check(t, "choices", len(got), len(want))
	for i := range want {
		check(t, fmt.Sprintf("choice %d", i), got[i], want[i])
	}
}

func TestLocalPortFor(t *testing.T) {
	tests := []struct{ remote, want int32 }{
		// Below 1024, only root may listen: moved above
		{80, 8080},
		{443, 8443},
		{1023, 9023},
		// Unprivileged: the same number on both sides
		{1024, 1024},
		{3000, 3000},
	}
	for _, tt := range tests {
		check(t, fmt.Sprintf("LocalPortFor(%d)", tt.remote), LocalPortFor(tt.remote), tt.want)
	}
}
