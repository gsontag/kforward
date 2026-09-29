package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"uuid"

	"github.com/adrg/xdg"
)

// ErrNotFound is returned for a UUID that is not in the configuration.
var ErrNotFound = errors.New("forward not found")

// Store holds the configuration in memory and persists it to a JSON file.
// It is safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	path   string
	config *Config
	// raw is the content last read or written: a reload of the same content
	// changes nothing
	raw []byte
}

// DefaultPath returns the XDG path of the configuration file,
// creating its parent directory if needed.
func DefaultPath() (string, error) {
	path, err := xdg.ConfigFile("kforward/config.json")
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return path, nil
}

// NewStore returns a store holding the default configuration.
// It does not read the file: call Load for that.
func NewStore(path string) *Store {
	return &Store{path: path, config: DefaultConfig()}
}

// Load reads and validates the configuration file. A missing or empty file
// yields the default configuration. On error, the previous configuration is kept.
func (s *Store) Load() error {
	cfg, raw, err := readConfig(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.config, s.raw = cfg, raw
	s.mu.Unlock()
	return nil
}

// readConfig returns the configuration of the file, and its raw content.
func readConfig(path string) (*Config, []byte, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultConfig(), nil, nil
	}

	if err != nil {
		return nil, nil, fmt.Errorf("read config: %w", err)
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return DefaultConfig(), buf, nil
	}

	var cfg Config
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid %s: %w", path, err)
	}

	if cfg.Forwards == nil {
		cfg.Forwards = []Forward{}
	}
	return &cfg, buf, nil
}

// Kubeconfig returns the configured kubeconfig path.
// An empty path means the standard kubectl loading rules.
func (s *Store) Kubeconfig() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Kubeconfig
}

// Forwards returns a copy of the configured forwards.
func (s *Store) Forwards() []Forward {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.config.Forwards)
}

// SetKubeconfig changes the kubeconfig path and saves the file atomically.
// On error, neither the file nor the in-memory configuration is modified.
func (s *Store) SetKubeconfig(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := *s.config
	next.Kubeconfig = path
	return s.commitLocked(&next)
}

// commitLocked writes next, in canonical order, and makes it the current
// configuration once written; s.mu must be held.
func (s *Store) commitLocked(next *Config) error {
	// A copy of Config shares its slice with the current one: sort a copy
	next.Forwards = slices.Clone(next.Forwards)
	Sort(next.Forwards)
	raw, err := writeConfig(s.path, next)
	if err != nil {
		return err
	}
	s.config, s.raw = next, raw
	return nil
}

// SaveForward adds f, or replaces the forward with the same UUID, and saves
// the file atomically. A forward without UUID is new: it gets one, and the
// saved forward is returned. On error, nothing is modified.
func (s *Store) SaveForward(f Forward) (Forward, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f.UUID == "" {
		f.UUID = uuid.New().String()
	}
	next := *s.config
	next.Forwards = slices.Clone(s.config.Forwards)
	if i := slices.IndexFunc(next.Forwards, func(e Forward) bool { return e.UUID == f.UUID }); i >= 0 {
		next.Forwards[i] = f
	} else {
		next.Forwards = append(next.Forwards, f)
	}
	if err := s.commitLocked(&next); err != nil {
		return Forward{}, err
	}
	return f, nil
}

// DeleteForward removes the forward with the given UUID and saves the file
// atomically. On error, nothing is modified.
func (s *Store) DeleteForward(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := *s.config
	next.Forwards = slices.DeleteFunc(
		slices.Clone(s.config.Forwards),
		func(e Forward) bool { return e.UUID == id },
	)
	if len(next.Forwards) == len(s.config.Forwards) {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return s.commitLocked(&next)
}

// writeConfig writes cfg atomically and returns the content written.
func writeConfig(path string, cfg *Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("refusing to save invalid config: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, fmt.Errorf("replace config: %w", err)
	}
	return data, nil
}

// ErrRemoved is returned by Reload when the file no longer exists.
var ErrRemoved = errors.New("configuration file removed")

// Reload reads the file again, like Load, and reports whether its content
// changed since it was last read or written: the saves of the store itself,
// and editors touching the file, change nothing. Unlike Load, a missing file
// is an error: the current configuration is kept rather than emptied.
func (s *Store) Reload() (changed bool, err error) {
	if _, err := os.Stat(s.path); errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("%s: %w", s.path, ErrRemoved)
	}
	cfg, raw, err := readConfig(s.path)
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if bytes.Equal(raw, s.raw) {
		return false, nil
	}
	s.config, s.raw = cfg, raw
	return true, nil
}
