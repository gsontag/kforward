package manager

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/forward"
	"github.com/gsontag/kforward/internal/kube"
)

// logBuffer collects the log lines; the runs write from their goroutines.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Split(strings.TrimSuffix(b.buf.String(), "\n"), "\n")
}

// newLogger returns a text logger into a buffer, without the time: the lines
// are compared as a whole.
func newLogger() (*slog.Logger, *logBuffer) {
	buf := &logBuffer{}
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	return slog.New(h), buf
}

func checkLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestLogLine(t *testing.T) {
	lost := errors.New("lost connection to pod")
	tests := []struct {
		name   string
		status forward.Status
		want   string
	}{
		{
			"stopped",
			forward.Status{State: forward.Stopped},
			`level=INFO msg="forward state" forward=grafana state=stopped`,
		},
		{
			"active",
			forward.Status{State: forward.Active, Endpoint: kube.Endpoint{Pod: "grafana-0", Port: 3000}},
			`level=INFO msg="forward state" forward=grafana state=active pod=grafana-0 port=3000`,
		},
		{
			"reconnecting",
			forward.Status{State: forward.Connecting, Failures: 2, Err: lost},
			`level=WARN msg="forward state" forward=grafana state=connecting failures=2 ` +
				`err="lost connection to pod"`,
		},
		// Active again after a reconnection: the old error is still worth a warning
		{
			"active after an error",
			forward.Status{State: forward.Active, Endpoint: kube.Endpoint{Pod: "grafana-1", Port: 3000}, Err: lost},
			`level=WARN msg="forward state" forward=grafana state=active pod=grafana-1 port=3000 ` +
				`err="lost connection to pod"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, buf := newLogger()
			m := New(nil, forward.DefaultPolicy, logger, func() {})

			m.log(config.Forward{Name: "grafana"}, tt.status)

			checkLines(t, buf.lines(), tt.want)
		})
	}
}

func TestLogStartStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logger, buf := newLogger()
		m, _, _ := newManagerLogging(t, logger)
		m.Load([]config.Forward{fwd("a", 1), fwd("b", 2)})

		mustDo(t, m.Start, "a")
		settle()
		mustDo(t, m.Stop, "a")
		// b never ran: stopping it again changes nothing, nothing to log
		m.StopAll()
		settle()

		checkLines(t, buf.lines(),
			`level=INFO msg="forward state" forward=a state=connecting`,
			`level=INFO msg="forward state" forward=a state=active pod=a-pod port=0`,
			`level=INFO msg="forward state" forward=a state=stopped`,
		)
	})
}

// The cancelled run reports Stopped after the new one started: that report
// is ignored, so it must not reach the log either.
func TestLogIgnoresStaleReports(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logger, buf := newLogger()
		m, _, _ := newManagerLogging(t, logger)
		m.Load([]config.Forward{fwd("a", 1)})
		mustDo(t, m.Start, "a")
		settle()

		mustDo(t, m.Stop, "a")
		mustDo(t, m.Start, "a")
		settle()

		checkLines(t, buf.lines(),
			`level=INFO msg="forward state" forward=a state=connecting`,
			`level=INFO msg="forward state" forward=a state=active pod=a-pod port=0`,
			`level=INFO msg="forward state" forward=a state=stopped`,
			`level=INFO msg="forward state" forward=a state=connecting`,
			`level=INFO msg="forward state" forward=a state=active pod=a-pod port=0`,
		)
	})
}

func TestLogFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logger, buf := newLogger()
		m, c, _ := newManagerLogging(t, logger)
		c.mu.Lock()
		c.failBuild["a"] = errors.New(`unknown context "nope"`)
		c.mu.Unlock()
		c.setFailResolve("b", errors.New("service not found"))
		m.Load([]config.Forward{fwd("a", 1), fwd("b", 2)})

		mustDo(t, m.Start, "a")
		mustDo(t, m.Start, "b")
		settle()

		checkLines(t, buf.lines(),
			`level=WARN msg="forward state" forward=a state=failed err="unknown context \"nope\""`,
			`level=INFO msg="forward state" forward=b state=connecting`,
			`level=WARN msg="forward state" forward=b state=failed failures=1 err="service not found"`,
		)
	})
}
