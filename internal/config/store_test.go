package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const validConfig = `{
	"kubeconfig": "/some/kubeconfig",
	"forwards": [
		{"uuid": "1", "name": "grafana", "target": "svc/grafana", "local-port": 3000, "remote-port": 80}
	]
}`

// writeFile writes a configuration file, or fails the test.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// loadStore returns a store loaded from validConfig.
func loadStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, path, validConfig)
	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s, path
}

func TestSaveThenLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := NewStore(path).SetKubeconfig("/some/kubeconfig"); err != nil {
		t.Fatalf("SetKubeconfig: %v", err)
	}

	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	check(t, "Kubeconfig", s.Kubeconfig(), "/some/kubeconfig")
}

func TestLoadDefaultConfig(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	check(t, "Kubeconfig", s.Kubeconfig(), "")
	if s.Forwards() == nil {
		t.Errorf("Forwards is nil")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"vide", ""},
		{"blancs", " \n\t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			writeFile(t, path, tt.content)

			s := NewStore(path)
			if err := s.Load(); err != nil {
				t.Fatalf("Load: %v", err)
			}
			check(t, "Kubeconfig", s.Kubeconfig(), "")
			check(t, "Forwards size", len(s.Forwards()), 0)
		})
	}
}

func TestLoadInvalidKeepsPrevious(t *testing.T) {
	s, path := loadStore(t)

	writeFile(t, path, `{"forwards": [`)
	err := s.Load()
	if err == nil {
		t.Fatal("Load: got nil error, want a parse error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q should mention the file path", err)
	}

	check(t, "Kubeconfig", s.Kubeconfig(), "/some/kubeconfig")
	check(t, "Forwards size", len(s.Forwards()), 1)
}

func TestForwardsReturnsCopy(t *testing.T) {
	s, _ := loadStore(t)

	fws := s.Forwards()
	fws[0].Name = "changed"

	check(t, "Name", s.Forwards()[0].Name, "grafana")
}

func TestSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := NewStore(filepath.Join(dir, "config.json")).SetKubeconfig("/k"); err != nil {
		t.Fatalf("SetKubeconfig: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	check(t, "files", strings.Join(names, ","), "config.json")
}

func TestSaveKeepsForwards(t *testing.T) {
	s, path := loadStore(t)
	if err := s.SetKubeconfig("/other"); err != nil {
		t.Fatalf("SetKubeconfig: %v", err)
	}

	reloaded := NewStore(path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	check(t, "Kubeconfig", reloaded.Kubeconfig(), "/other")
	check(t, "Forwards size", len(reloaded.Forwards()), 1)
}

// Only fails under go test -race: without the lock, the detector reports
// the concurrent access to s.config.
func TestConcurrentAccess(t *testing.T) {
	s, _ := loadStore(t)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { _ = s.Load() })
		wg.Go(func() { _ = s.Forwards() })
		wg.Go(func() { _ = s.SetKubeconfig("/k") })
	}
	wg.Wait()
}

func TestLoadInvalidConfigKeepsPrevious(t *testing.T) {
	s, path := loadStore(t)

	// JSON correct, mais le forward n'a pas de port local
	writeFile(
		t,
		path,
		`{"forwards": [{"uuid": "1", "name": "x", "target": "svc/x", "remote-port": 80}]}`,
	)
	err := s.Load()
	if err == nil {
		t.Fatal("Load: got nil error, want a validation error")
	}
	if !strings.Contains(err.Error(), "local-port is required") {
		t.Errorf("got error %v, want it to mention the local port", err)
	}

	check(t, "Name", s.Forwards()[0].Name, "grafana")
}

func TestWriteRefusesInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Forwards: []Forward{{UUID: "1"}}}

	if err := writeConfig(path, cfg); err == nil {
		t.Fatal("writeConfig: got nil error, want a validation error")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("config file should not exist, Stat error: %v", err)
	}
}
