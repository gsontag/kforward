package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
	"gsontag.fr/kforward/internal/kube"
)

// portRelease is how long a fake forward keeps its local port after being
// cancelled, like a real listener being closed.
const portRelease = 100 * time.Millisecond

// cluster is a fake cluster: it hands out connectors and records what they do.
type cluster struct {
	mu sync.Mutex
	// held are the local ports in use.
	held map[uint16]bool
	// built counts the connectors built per forward UUID.
	built map[string]int
	// failBuild and failResolve make the matching forward fail.
	failBuild   map[string]error
	failResolve map[string]error
}

func newCluster() *cluster {
	return &cluster{
		held:        map[uint16]bool{},
		built:       map[string]int{},
		failBuild:   map[string]error{},
		failResolve: map[string]error{},
	}
}

func (c *cluster) factory(f config.Forward) (forward.Connector, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.failBuild[f.UUID]; err != nil {
		return nil, err
	}
	c.built[f.UUID]++
	return &fakeConnector{cluster: c, forward: f}, nil
}

func (c *cluster) builds(uuid string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.built[uuid]
}

func (c *cluster) holds(port uint16) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.held[port]
}

func (c *cluster) heldPorts() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.held)
}

func (c *cluster) setFailResolve(uuid string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failResolve[uuid] = err
}

type fakeConnector struct {
	cluster *cluster
	forward config.Forward
}

func (c *fakeConnector) Resolve(context.Context) (kube.Endpoint, error) {
	c.cluster.mu.Lock()
	defer c.cluster.mu.Unlock()
	if err := c.cluster.failResolve[c.forward.UUID]; err != nil {
		return kube.Endpoint{}, forward.Permanent(err)
	}
	return kube.Endpoint{Pod: c.forward.UUID + "-pod"}, nil
}

func (c *fakeConnector) Forward(ctx context.Context, _ kube.Endpoint, onReady func()) error {
	port := c.forward.LocalPort
	c.cluster.mu.Lock()
	if c.cluster.held[port] {
		c.cluster.mu.Unlock()
		return forward.Permanent(fmt.Errorf("local port %d busy", port))
	}
	c.cluster.held[port] = true
	c.cluster.mu.Unlock()

	onReady()
	<-ctx.Done()
	time.Sleep(portRelease)

	c.cluster.mu.Lock()
	delete(c.cluster.held, port)
	c.cluster.mu.Unlock()
	return nil
}

func fwd(uuid string, port uint16) config.Forward {
	return config.Forward{
		UUID: uuid, Name: uuid, Target: "svc/" + uuid,
		LocalPort: port, RemotePort: intstr.FromInt32(80),
	}
}

func autoStarted(f config.Forward) config.Forward {
	f.AutoStart = true
	return f
}

// newManager returns a manager on a fake cluster; it counts the calls to
// onChange, and closes the manager at the end of the test.
func newManager(t *testing.T) (*Manager, *cluster, func() int) {
	t.Helper()
	c := newCluster()
	var mu sync.Mutex
	changes := 0
	var m *Manager
	m = New(c.factory, forward.DefaultPolicy, func() {
		// Reading the state from onChange, as the UI will: must not deadlock
		_ = m.Snapshot()
		mu.Lock()
		changes++
		mu.Unlock()
	})
	t.Cleanup(m.Close)
	return m, c, func() int {
		mu.Lock()
		defer mu.Unlock()
		return changes
	}
}

// states summarizes the snapshot as "uuid=state" in configuration order.
func states(m *Manager) string {
	entries := m.Snapshot()
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.Forward.UUID+"="+e.Status.State.String())
	}
	return strings.Join(parts, " ")
}

// settle lets every goroutine of the bubble run, and fake ports be released.
func settle() {
	time.Sleep(time.Second)
	synctest.Wait()
}

func TestFirstLoadAutoStarts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, _, changes := newManager(t)

		m.Load([]config.Forward{autoStarted(fwd("a", 1)), fwd("b", 2)})
		settle()

		check(t, "states", states(m), "a=active b=stopped")
		if changes() == 0 {
			t.Error("onChange not called")
		}
	})
}

func TestStartStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, c, _ := newManager(t)
		m.Load([]config.Forward{fwd("a", 1)})

		mustDo(t, m.Start, "a")
		mustDo(t, m.Start, "a") // already running: no second connector
		settle()
		check(t, "after start", states(m), "a=active")
		check(t, "connectors built", c.builds("a"), 1)

		mustDo(t, m.Stop, "a")
		// Stopped at once, before the fake port is even released
		check(t, "right after stop", states(m), "a=stopped")
		mustDo(t, m.Stop, "a")
		settle()
		check(t, "after stop", states(m), "a=stopped")
	})
}

func mustDo(t *testing.T, action func(string) error, uuid string) {
	t.Helper()
	if err := action(uuid); err != nil {
		t.Fatalf("%s: %v", uuid, err)
	}
}

func TestUnknownForward(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, _, _ := newManager(t)
		m.Load([]config.Forward{fwd("a", 1)})

		for name, action := range map[string]func(string) error{"Start": m.Start, "Stop": m.Stop} {
			if err := action("nope"); !errors.Is(err, ErrUnknownForward) {
				t.Errorf("%s(nope) = %v, want ErrUnknownForward", name, err)
			}
		}
		// The manager must still answer afterwards
		mustDo(t, m.Start, "a")
		settle()
		check(t, "states", states(m), "a=active")
	})
}

func TestRestartWaitsForThePort(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, c, _ := newManager(t)
		m.Load([]config.Forward{fwd("a", 1)})
		mustDo(t, m.Start, "a")
		settle()

		// A double click on the switch: the old run still holds the port
		mustDo(t, m.Stop, "a")
		mustDo(t, m.Start, "a")
		settle()

		check(t, "states", states(m), "a=active")
		check(t, "connectors built", c.builds("a"), 2)
		if err := m.Snapshot()[0].Status.Err; err != nil {
			t.Errorf("unexpected error %v", err)
		}
	})
}

func TestStaleReportIgnored(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, _, _ := newManager(t)
		m.Load([]config.Forward{fwd("a", 1)})
		mustDo(t, m.Start, "a")
		settle()

		mustDo(t, m.Stop, "a")
		mustDo(t, m.Start, "a")
		// The old run reports Stopped once its port is released, after the
		// new run started: that report must not win
		settle()
		time.Sleep(time.Minute)
		synctest.Wait()

		check(t, "states", states(m), "a=active")
	})
}

func TestReload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, c, _ := newManager(t)
		m.Load([]config.Forward{fwd("renamed", 1), fwd("moved", 2), fwd("idle", 3), fwd("removed", 4)})
		for _, uuid := range []string{"renamed", "moved", "removed"} {
			mustDo(t, m.Start, uuid)
		}
		settle()

		renamed := fwd("renamed", 1)
		renamed.Name, renamed.Group, renamed.URL = "Grafana", "Monitoring", "http://localhost:1"
		movedRunning := fwd("moved", 20)
		movedIdle := fwd("idle", 30)
		added := autoStarted(fwd("added", 5))
		m.Load([]config.Forward{added, movedIdle, movedRunning, renamed})
		settle()

		// Order of the new configuration; AutoStart only applies to the first load
		check(t, "states", states(m), "added=stopped idle=stopped moved=active renamed=active")
		check(t, "renamed kept its connector", c.builds("renamed"), 1)
		check(t, "moved restarted", c.builds("moved"), 2)
		check(t, "idle not started", c.builds("idle"), 0)
		check(t, "new name visible", m.Snapshot()[3].Forward.Name, "Grafana")
		check(t, "removed port released", c.holds(4), false)
	})
}

func TestFactoryFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, c, _ := newManager(t)
		c.mu.Lock()
		c.failBuild["a"] = errors.New(`unknown context "nope"`)
		c.mu.Unlock()
		m.Load([]config.Forward{fwd("a", 1)})

		mustDo(t, m.Start, "a")
		status := m.Snapshot()[0].Status
		check(t, "state", status.State, forward.Failed)
		if status.Err == nil || !strings.Contains(status.Err.Error(), "unknown context") {
			t.Errorf("error = %v, want the factory error", status.Err)
		}

		// Once the configuration is fixed, the switch works again
		c.mu.Lock()
		delete(c.failBuild, "a")
		c.mu.Unlock()
		mustDo(t, m.Start, "a")
		settle()
		check(t, "states", states(m), "a=active")
	})
}

func TestStartAfterGiveUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, c, _ := newManager(t)
		c.setFailResolve("a", errors.New("service not found"))
		m.Load([]config.Forward{fwd("a", 1)})

		mustDo(t, m.Start, "a")
		settle()
		check(t, "after give-up", states(m), "a=failed")

		c.setFailResolve("a", nil)
		mustDo(t, m.Start, "a")
		settle()
		check(t, "after restart", states(m), "a=active")
	})
}

func TestStopAll(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, _, _ := newManager(t)
		m.Load([]config.Forward{autoStarted(fwd("a", 1)), autoStarted(fwd("b", 2))})
		settle()

		m.StopAll()

		check(t, "states", states(m), "a=stopped b=stopped")
	})
}

func TestClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, c, _ := newManager(t)
		m.Load([]config.Forward{autoStarted(fwd("a", 1)), autoStarted(fwd("b", 2))})
		settle()

		start := time.Now()
		m.Close()
		// Close waits for the runs: the fake ports are released on return
		if elapsed := time.Since(start); elapsed < portRelease {
			t.Errorf("Close returned after %s, before the ports were released", elapsed)
		}
		check(t, "ports held", c.heldPorts(), 0)

		mustDo(t, m.Start, "a")
		settle()
		check(t, "start after close", states(m), "a=stopped b=stopped")
	})
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
