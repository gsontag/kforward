package config

import (
	"os"
	"path/filepath"
)

type Forward struct {
	UUID       string  `json:"uuid"`
	Name       string  `json:"name"`
	Group      string  `json:"group"`
	Context    string  `json:"context"`
	Namespace  string  `json:"namespace"`
	Target     string  `json:"target"`
	LocalPort  uint16  `json:"local-port"`
	RemotePort uint16  `json:"remote-port"`
	Url        *string `json:"url"`
	AutoStart  bool    `json:"auto-start"`
}

type Config struct {
	Kubeconfig string `json:"Kubeconfig"`
	Forwards   []Forward
}

func DefaultConfig() *Config {
	kubeconfig := ""
	if home, err := os.UserHomeDir(); err == nil {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	return &Config{
		Kubeconfig: kubeconfig,
		Forwards:   make([]Forward, 0),
	}
}
