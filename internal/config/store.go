package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

type Store struct {
	config *Config
	path   string
}

func NewConfigStore() (*Store, error) {
	path, err := xdg.ConfigFile("forward-it/config.json")
	if err != nil {
		return nil, fmt.Errorf("could not resolve path for config file: %w", err)
	}

	store := Store{
		path: path,
	}
	if err := store.Reload(); err != nil {
		return nil, err
	}

	return &store, nil
}

func (s *Store) Reload() error {
	_, err := os.Stat(s.path)
	if os.IsNotExist(err) {
		s.config = DefaultConfig()
		return nil
	}

	dir, fileName := filepath.Split(s.path)
	if len(dir) == 0 {
		dir = "."
	}

	buf, err := fs.ReadFile(os.DirFS(dir), fileName)
	if err != nil {
		return fmt.Errorf("could not read the configuration file: %w", err)
	}

	if len(buf) == 0 {
		s.config = DefaultConfig()
		return nil
	}

	cfg := Config{}
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return fmt.Errorf("configuration file does not have a valid format: %w", err)
	}

	s.config = &cfg
	return nil
}

func (s *Store) Save() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}

func (s *Store) GetKubeconfig() string {
	return s.config.Kubeconfig
}

func (s *Store) GetForwards() []Forward {
	return s.config.Forwards
}

func (s *Store) SetKubeconfig(file string) error {
	s.config.Kubeconfig = file
	return s.Save()
}
