// Package manager drives all the configured forwards: it starts and stops
// them on demand, applies configuration reloads and reports their states.
package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
)

// ConnectorFactory builds the connector of a configured forward.
type ConnectorFactory func(config.Forward) (forward.Connector, error)

// Entry is a snapshot of one managed forward.
type Entry struct {
	Forward config.Forward
	Status  forward.Status
}

// ErrUnknownForward is returned for a UUID that is not in the configuration.
var ErrUnknownForward = errors.New("unknown forward")

// Manager is safe for concurrent use.
type Manager struct {
	newConnector ConnectorFactory
	policy       forward.Policy
	onChange     func()

	mu      sync.Mutex
	entries []*entry
	loaded  bool
	closed  bool
	running sync.WaitGroup
}

type entry struct {
	forward config.Forward
	status  forward.Status
	// run is the current run, nil when the forward is not wanted.
	run *run
	// done is closed once the latest run has returned and freed its local port.
	done <-chan struct{}
}

type run struct {
	cancel context.CancelFunc
}

// connectionSettings are the fields that define the connection: changing
// any of them requires a restart.
type connectionSettings struct {
	context, namespace, target, address string
	localPort                           uint16
	remotePort                          string
}

// New returns a manager without forwards: call Load. onChange is called,
// from any goroutine, after every change visible in Snapshot.
func New(newConnector ConnectorFactory, policy forward.Policy, onChange func()) *Manager {
	return &Manager{newConnector: newConnector, policy: policy, onChange: onChange}
}

// Load applies a configuration. Forwards whose connection settings are
// unchanged keep running; changed ones restart; removed ones stop. On the
// first load only, the forwards marked AutoStart start.
func (m *Manager) Load(forwards []config.Forward) {
	m.mu.Lock()
	previous := make(map[string]*entry, len(m.entries))
	for _, e := range m.entries {
		previous[e.forward.UUID] = e
	}

	entries := make([]*entry, 0, len(forwards))
	for _, f := range forwards {
		e, found := previous[f.UUID]
		delete(previous, f.UUID)
		switch {
		case !found:
			e = &entry{forward: f}
			if !m.loaded && f.AutoStart {
				m.startLocked(e)
			}
		case connection(e.forward) != connection(f):
			wanted := e.run != nil
			m.stopLocked(e)
			e.forward = f
			if wanted {
				m.startLocked(e)
			}
		default:
			e.forward = f
		}
		entries = append(entries, e)
	}
	for _, e := range previous {
		m.stopLocked(e)
	}
	m.entries = entries
	m.loaded = true
	m.mu.Unlock()
	m.onChange()
}

func connection(f config.Forward) connectionSettings {
	return connectionSettings{
		context:    f.Context,
		namespace:  f.Namespace,
		target:     f.Target,
		address:    f.BindAddress(),
		localPort:  f.LocalPort,
		remotePort: f.RemotePort.String(),
	}
}

// Start starts the forward, unless it already runs.
func (m *Manager) Start(uuid string) error {
	return m.apply(uuid, m.startLocked)
}

// Stop stops the forward, unless it is already stopped.
func (m *Manager) Stop(uuid string) error {
	return m.apply(uuid, m.stopLocked)
}

// StopAll stops every forward.
func (m *Manager) StopAll() {
	m.mu.Lock()
	for _, e := range m.entries {
		m.stopLocked(e)
	}
	m.mu.Unlock()
	m.onChange()
}

// Close stops every forward and waits until they have all released their
// local ports. The manager is unusable afterwards.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, e := range m.entries {
		m.stopLocked(e)
	}
	m.mu.Unlock()
	m.running.Wait()
}

// Snapshot returns the forwards in configuration order.
func (m *Manager) Snapshot() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := make([]Entry, len(m.entries))
	for i, e := range m.entries {
		entries[i] = Entry{Forward: e.forward, Status: e.status}
	}
	return entries
}

func (m *Manager) apply(uuid string, action func(*entry)) error {
	m.mu.Lock()
	var target *entry
	for _, e := range m.entries {
		if e.forward.UUID == uuid {
			target = e
			break
		}
	}
	if target == nil {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrUnknownForward, uuid)
	}
	action(target)
	m.mu.Unlock()
	m.onChange()
	return nil
}

func (m *Manager) startLocked(e *entry) {
	if e.run != nil || m.closed {
		return
	}
	connector, err := m.newConnector(e.forward)
	if err != nil {
		e.status = forward.Status{State: forward.Failed, Err: err}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &run{cancel: cancel}
	previous := e.done
	done := make(chan struct{})
	e.run, e.done = r, done

	m.running.Go(func() {
		defer close(done)
		// A stopping run may still hold the local port
		if previous != nil {
			<-previous
		}
		forward.Run(ctx, connector, m.policy, func(s forward.Status) { m.report(e, r, s) })
	})
}

func (m *Manager) stopLocked(e *entry) {
	if e.run == nil {
		return
	}
	e.run.cancel()
	e.run = nil
	// Stopped at once from the user's point of view; the reports of the
	// cancelled run are ignored from now on
	e.status = forward.Status{State: forward.Stopped}
}

func (m *Manager) report(e *entry, r *run, s forward.Status) {
	m.mu.Lock()
	if e.run != r {
		m.mu.Unlock()
		return
	}
	e.status = s
	if s.State == forward.Failed {
		// Run has given up: the forward is no longer wanted
		e.run = nil
	}
	m.mu.Unlock()
	m.onChange()
}
