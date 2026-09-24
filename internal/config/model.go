package config

import "k8s.io/apimachinery/pkg/util/intstr"

type Forward struct {
	UUID       string             `json:"uuid"`
	Name       string             `json:"name"`
	Group      string             `json:"group,omitempty"`
	Context    string             `json:"context,omitempty"`
	Namespace  string             `json:"namespace,omitempty"`
	Target     string             `json:"target"`
	LocalPort  uint16             `json:"local-port"`
	RemotePort intstr.IntOrString `json:"remote-port"`
	Address    string             `json:"address,omitempty"`
	URL        string             `json:"url,omitempty"`
	AutoStart  bool               `json:"auto-start"`
}

type Config struct {
	Kubeconfig string    `json:"kubeconfig,omitempty"`
	Forwards   []Forward `json:"forwards"`
}

func (f *Forward) BindAddress() string {
	if f.Address == "" {
		return "127.0.0.1"
	}
	return f.Address
}

func DefaultConfig() *Config {
	return &Config{
		Forwards: []Forward{},
	}
}
