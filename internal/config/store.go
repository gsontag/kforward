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

	"github.com/adrg/xdg"
)

type Store struct {
	mu     sync.RWMutex
	path   string
	config *Config
}

func DefaultPath() (string, error) {
	path, err := xdg.ConfigFile("kforward/config.json")
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return path, nil
}

func NewStore(path string) *Store {
	return &Store{path: path, config: DefaultConfig()}
}

func (s *Store) Load() error {
	cfg, err := readConfig(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.config = cfg
	s.mu.Unlock()
	return nil
}

func readConfig(path string) (*Config, error) {
	buf, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultConfig(), nil
	}

	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return DefaultConfig(), nil
	}

	var cfg Config
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}

	if cfg.Forwards == nil {
		cfg.Forwards = []Forward{}
	}
	return &cfg, nil
}

func (s *Store) Kubeconfig() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Kubeconfig
}

func (s *Store) Forwards() []Forward {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.config.Forwards)
}

func (s *Store) SetKubeconfig(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := *s.config
	next.Kubeconfig = path
	if err := writeConfig(s.path, &next); err != nil {
		return err
	}
	s.config = &next
	return nil
}

func writeConfig(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("refusing to save invalid config: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
