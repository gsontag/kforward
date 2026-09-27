//go:build integration

package forward_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/kube"
)

// stateTimeout bounds each wait for a status: a pod start or a reconnection
// takes a few seconds on kind.
const stateTimeout = 30 * time.Second

// running is a forward.Run started in the background by a test.
type running struct {
	statuses chan forward.Status
	cancel   context.CancelFunc
	done     chan struct{}
}

// startForward runs the forward f until the end of the test.
func (e *itEnv) startForward(t *testing.T, f config.Forward) *running {
	t.Helper()
	connector, err := forward.NewClusterConnector(e.client, f)
	if err != nil {
		t.Fatalf("NewClusterConnector: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	r := &running{
		// Large enough for Run never to block on a test that stopped reading
		statuses: make(chan forward.Status, 64),
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go func() {
		defer close(r.done)
		forward.Run(
			ctx,
			connector,
			forward.DefaultPolicy,
			func(s forward.Status) { r.statuses <- s },
		)
	}()
	t.Cleanup(r.stop)
	return r
}

// stop cancels the forward and waits for Run to return.
func (r *running) stop() {
	r.cancel()
	<-r.done
}

// waitFor skips the statuses until one in state want, and fails the test if
// the forward gives up first or nothing comes in time.
func (r *running) waitFor(t *testing.T, want forward.State) forward.Status {
	t.Helper()
	timeout := time.After(stateTimeout)
	for {
		select {
		case s := <-r.statuses:
			if s.State == want {
				return s
			}
			if s.State == forward.Failed {
				t.Fatalf("forward failed while waiting for %s: %v", want, s.Err)
			}
		case <-timeout:
			t.Fatalf("no %s status within %s", want, stateTimeout)
		}
	}
}

// appForward is a forward to the test app through its service, on a free local port.
func (e *itEnv) appForward(t *testing.T) config.Forward {
	t.Helper()
	return config.Forward{
		UUID:       "it",
		Name:       t.Name(),
		Namespace:  e.namespace,
		Target:     "svc/" + appName,
		LocalPort:  freeLocalPort(t),
		RemotePort: intstr.FromString("web"),
	}
}

func freeLocalPort(t *testing.T) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() { _ = listener.Close() }()
	return uint16(listener.Addr().(*net.TCPAddr).Port)
}

// A new TCP connection for every request: each one is a new stream of the forward
var httpClient = &http.Client{
	Timeout:   5 * time.Second,
	Transport: &http.Transport{DisableKeepAlives: true},
}

// podBehind returns the name of the pod that answers on the local port.
func podBehind(t *testing.T, port uint16) string {
	t.Helper()
	resp, err := httpClient.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("GET through the forward: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return strings.TrimSpace(string(body))
}

func TestIntegrationForward(t *testing.T) {
	e := requireCluster(t)
	f := e.appForward(t)
	r := e.startForward(t, f)

	active := r.waitFor(t, forward.Active)

	// Several connections in a row share the same stream to the API server
	for i := range 3 {
		if got := podBehind(t, f.LocalPort); got != active.Endpoint.Pod {
			t.Errorf("request %d answered by %q, want %q", i, got, active.Endpoint.Pod)
		}
	}

	r.stop()
	if s := <-r.statuses; s.State != forward.Stopped {
		t.Errorf("final status = %s, want stopped", s.State)
	}
	// Once stopped, nothing listens on the local port any more
	if _, err := httpClient.Get(fmt.Sprintf("http://127.0.0.1:%d/", f.LocalPort)); err == nil {
		t.Error("the local port still answers after the stop")
	}
}

func TestIntegrationReconnectsAfterPodDeletion(t *testing.T) {
	e := requireCluster(t)
	f := e.appForward(t)
	r := e.startForward(t, f)

	first := r.waitFor(t, forward.Active)
	e.deletePod(t, first.Endpoint.Pod)

	reconnecting := r.waitFor(t, forward.Connecting)
	if !errors.Is(reconnecting.Err, kube.ErrPodGone) {
		t.Errorf("reconnection error = %v, want it to wrap kube.ErrPodGone", reconnecting.Err)
	}

	second := r.waitFor(t, forward.Active)
	if second.Endpoint.Pod == first.Endpoint.Pod {
		t.Fatalf("reconnected to the deleted pod %s", first.Endpoint.Pod)
	}
	if got := podBehind(t, f.LocalPort); got != second.Endpoint.Pod {
		t.Errorf("answered by %q, want the new pod %q", got, second.Endpoint.Pod)
	}
}

func TestIntegrationGivesUpWhenAppScaledDown(t *testing.T) {
	e := requireCluster(t)
	f := e.appForward(t)
	r := e.startForward(t, f)
	r.waitFor(t, forward.Active)

	e.scaleApp(t, 0)

	failed := r.waitFor(t, forward.Failed)
	if forward.IsPermanent(failed.Err) {
		t.Errorf("error %v should be transient: the app may come back", failed.Err)
	}
	if !strings.Contains(failed.Err.Error(), "no ready pod") {
		t.Errorf("final error = %v, want the missing pod as the reason", failed.Err)
	}
	if want := forward.DefaultPolicy.MaxRetries + 1; failed.Failures != want {
		t.Errorf("gave up after %d failures, want %d", failed.Failures, want)
	}
}

func TestIntegrationPermanentErrors(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, f *config.Forward)
		wantErr string
	}{
		{"local port busy", func(t *testing.T, f *config.Forward) {
			listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", f.LocalPort))
			if err != nil {
				t.Fatalf("Listen: %v", err)
			}
			t.Cleanup(func() { _ = listener.Close() })
		}, "unavailable"},
		{
			"missing service",
			func(_ *testing.T, f *config.Forward) { f.Target = "svc/absent" },
			`"absent" not found`,
		},
		{
			"unknown port",
			func(_ *testing.T, f *config.Forward) { f.RemotePort = intstr.FromString("nope") },
			"port not found",
		},
	}

	e := requireCluster(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := e.appForward(t)
			tt.prepare(t, &f)
			r := e.startForward(t, f)
			start := time.Now()

			failed := r.waitFor(t, forward.Failed)
			if !forward.IsPermanent(failed.Err) ||
				!strings.Contains(failed.Err.Error(), tt.wantErr) {
				t.Errorf(
					"got error %v, want a permanent error containing %q",
					failed.Err,
					tt.wantErr,
				)
			}
			// Permanent: no retry, hance no RetryDelay
			if elapsed := time.Since(start); elapsed >= forward.DefaultPolicy.RetryDelay {
				t.Errorf("gave up after %s, want at once", elapsed)
			}
		})
	}
}

func TestIntegrationStopWhileReconnecting(t *testing.T) {
	e := requireCluster(t)
	f := e.appForward(t)
	r := e.startForward(t, f)
	first := r.waitFor(t, forward.Active)

	e.deletePod(t, first.Endpoint.Pod)
	r.waitFor(t, forward.Connecting)

	start := time.Now()
	r.stop()
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("stop took %s: the pause before the reconnection must be interrupted", elapsed)
	}
	if s := <-r.statuses; s.State != forward.Stopped {
		t.Errorf("final status = %s, want stopped", s.State)
	}
}
