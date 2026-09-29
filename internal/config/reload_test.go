package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const otherConfig = `{
	"forwards": [
		{"uuid": "2", "name": "keycloak", "target": "svc/keycloak", "local-port": 8080, "remote-port": 8080}
	]
}`

func reloadOK(t *testing.T, s *Store) bool {
	t.Helper()
	changed, err := s.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	return changed
}

func TestReloadChangedByHand(t *testing.T) {
	s, path := loadStore(t)

	writeFile(t, path, otherConfig)

	check(t, "changed", reloadOK(t, s), true)
	check(t, "forward", s.Forwards()[0].Name, "keycloak")
}

func TestReloadSameContent(t *testing.T) {
	s, path := loadStore(t)

	// An editor saving without change: the file is written, not changed
	writeFile(t, path, validConfig)

	check(t, "changed", reloadOK(t, s), false)
}

// The saves of the application are seen by the file monitor too: reloading
// them must change nothing.
func TestReloadAfterOwnSave(t *testing.T) {
	s, _ := loadStore(t)
	if _, err := s.SaveForward(newForward("keycloak")); err != nil {
		t.Fatalf("SaveForward: %v", err)
	}

	check(t, "changed", reloadOK(t, s), false)
	check(t, "forwards", len(s.Forwards()), 2)
}

func TestReloadInvalidKeepsCurrent(t *testing.T) {
	s, path := loadStore(t)

	// Half written by an editor, or a typo
	writeFile(t, path, `{"forwards": [`)
	changed, err := s.Reload()

	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Errorf("got error %v, want a parse error", err)
	}
	check(t, "changed", changed, false)
	check(t, "forward kept", s.Forwards()[0].Name, "grafana")

	// Fixed back to what was loaded: no change, the broken content was never
	// kept as the current one
	writeFile(t, path, validConfig)
	check(t, "changed after the fix", reloadOK(t, s), false)
	writeFile(t, path, otherConfig)
	check(t, "changed to another content", reloadOK(t, s), true)
}

func TestReloadRemovedKeepsCurrent(t *testing.T) {
	s, path := loadStore(t)
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	changed, err := s.Reload()

	if !errors.Is(err, ErrRemoved) {
		t.Errorf("got error %v, want ErrRemoved", err)
	}
	check(t, "changed", changed, false)
	// Emptying the configuration would stop every forward
	check(t, "forwards kept", len(s.Forwards()), 1)
}

func TestReloadEmptiedFile(t *testing.T) {
	s, path := loadStore(t)

	// Unlike a removed file, an empty one is a choice: no forward at all
	writeFile(t, path, "")

	check(t, "changed", reloadOK(t, s), true)
	check(t, "forwards", len(s.Forwards()), 0)
}

func TestReloadAfterFailedLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, path, `{"forwards": [`)
	s := NewStore(path)
	if err := s.Load(); err == nil {
		t.Fatal("Load: got nil error, want a parse error")
	}

	writeFile(t, path, validConfig)

	check(t, "changed", reloadOK(t, s), true)
	check(t, "forward", s.Forwards()[0].Name, "grafana")
}
