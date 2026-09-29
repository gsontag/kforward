package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"uuid"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func newForward(name string) Forward {
	return Forward{
		Name:       name,
		Target:     "svc/" + name,
		LocalPort:  3000,
		RemotePort: intstr.FromInt32(80),
	}
}

// reload reads the file again in a new store: what was really written.
func reload(t *testing.T, path string) []Forward {
	t.Helper()
	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s.Forwards()
}

func TestSaveNewForward(t *testing.T) {
	s, path := loadStore(t)

	saved, err := s.SaveForward(newForward("keycloak"))
	if err != nil {
		t.Fatalf("SaveForward: %v", err)
	}
	if _, err := uuid.Parse(saved.UUID); err != nil {
		t.Errorf("UUID %q is not a UUID: %v", saved.UUID, err)
	}
	check(t, "name", saved.Name, "keycloak")

	onDisk := reload(t, path)
	check(t, "forwards on disk", len(onDisk), 2)
	check(t, "in memory", len(s.Forwards()), 2)
}

func TestSaveNewForwardsGetDistinctUUIDs(t *testing.T) {
	s, _ := loadStore(t)
	a, errA := s.SaveForward(newForward("a"))
	b, errB := s.SaveForward(newForward("b"))
	if err := errors.Join(errA, errB); err != nil {
		t.Fatalf("SaveForward: %v", err)
	}
	if a.UUID == b.UUID {
		t.Errorf("two new forwards share the UUID %s", a.UUID)
	}
}

func TestSaveExistingForward(t *testing.T) {
	s, path := loadStore(t)
	existing := s.Forwards()[0]
	existing.Name, existing.LocalPort = "grafana-renamed", 3001

	saved, err := s.SaveForward(existing)
	if err != nil {
		t.Fatalf("SaveForward: %v", err)
	}
	check(t, "UUID kept", saved.UUID, existing.UUID)

	onDisk := reload(t, path)
	// Replaced, not duplicated
	check(t, "forwards on disk", len(onDisk), 1)
	check(t, "name on disk", onDisk[0].Name, "grafana-renamed")
	check(t, "port on disk", onDisk[0].LocalPort, uint16(3001))
}

func TestSaveWritesCanonicalOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s := NewStore(path)
	for _, name := range []string{"zeta", "alpha", "Mid"} {
		if _, err := s.SaveForward(newForward(name)); err != nil {
			t.Fatalf("SaveForward(%s): %v", name, err)
		}
	}

	// The order of the file itself, not the one of a sorted reading
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	alpha, mid, zeta := strings.Index(
		text,
		`"alpha"`,
	), strings.Index(
		text,
		`"Mid"`,
	), strings.Index(
		text,
		`"zeta"`,
	)
	if alpha >= mid || mid >= zeta {
		t.Errorf("file not in canonical order:\n%s", text)
	}
}

func TestSaveInvalidForwardChangesNothing(t *testing.T) {
	s, path := loadStore(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	invalid := newForward("broken")
	invalid.LocalPort = 0
	if _, err := s.SaveForward(invalid); err == nil ||
		!strings.Contains(err.Error(), "local-port is required") {
		t.Fatalf("got error %v, want a validation error", err)
	}

	check(t, "in memory", len(s.Forwards()), 1)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	check(t, "file unchanged", string(after), string(before))
}

func TestDeleteForward(t *testing.T) {
	s, path := loadStore(t)
	id := s.Forwards()[0].UUID

	if err := s.DeleteForward(id); err != nil {
		t.Fatalf("DeleteForward: %v", err)
	}
	check(t, "in memory", len(s.Forwards()), 0)
	check(t, "on disk", len(reload(t, path)), 0)
}

func TestDeleteUnknownForward(t *testing.T) {
	s, path := loadStore(t)
	before, _ := os.ReadFile(path)

	err := s.DeleteForward("nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got error %v, want ErrNotFound", err)
	}
	check(t, "in memory", len(s.Forwards()), 1)
	after, _ := os.ReadFile(path)
	check(t, "file unchanged", string(after), string(before))
}

// Without the clone in commitLocked, the sort would reorder the current
// configuration even when the write fails.
func TestFailedWriteKeepsTheOrder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Written by hand, out of canonical order
	writeFile(t, path, `{"forwards": [
		{"uuid": "2", "name": "zeta", "target": "svc/zeta", "local-port": 3000, "remote-port": 80},
		{"uuid": "1", "name": "alpha", "target": "svc/alpha", "local-port": 3001, "remote-port": 80}
	]}`)
	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// No temporary file can be created: the write fails
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := s.SetKubeconfig("/other"); err == nil {
		t.Fatal("SetKubeconfig: got nil error, want a write error")
	}

	got := s.Forwards()
	check(t, "order kept", got[0].Name+","+got[1].Name, "zeta,alpha")
	check(t, "kubeconfig kept", s.Kubeconfig(), "")
}
