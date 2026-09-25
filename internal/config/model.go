package config

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
)

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

func (f *Forward) Validate() error {
	return errors.Join(f.problems()...)
}

func (f *Forward) problems() []error {
	var errs []error
	if strings.TrimSpace(f.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if strings.TrimSpace(f.Target) == "" {
		errs = append(errs, errors.New("target is required"))
	}
	if f.LocalPort == 0 {
		errs = append(errs, errors.New("local-port is required"))
	}
	if f.Address != "" && f.Address != "localhost" && net.ParseIP(f.Address) == nil {
		errs = append(errs, fmt.Errorf("address %q is not an IP address", f.Address))
	}
	return append(errs, remotePortProblems(f.RemotePort)...)
}

func remotePortProblems(p intstr.IntOrString) []error {
	var msgs []string
	switch p.Type {
	case intstr.Int:
		msgs = validation.IsValidPortNum(int(p.IntVal))
	case intstr.String:
		msgs = validation.IsValidPortName(p.StrVal)
	}
	errs := make([]error, 0, len(msgs))
	for _, m := range msgs {
		errs = append(errs, fmt.Errorf("remote-port: %s", m))
	}
	return errs
}

func DefaultConfig() *Config {
	return &Config{
		Forwards: []Forward{},
	}
}

func (c *Config) Validate() error {
	var errs []error
	seen := make(map[string]bool, len(c.Forwards))

	for i, f := range c.Forwards {
		label := fmt.Sprintf("forward #%d", i+1)
		if f.Name != "" {
			label += fmt.Sprintf(" (%s)", f.Name)
		}

		switch {
		case f.UUID == "":
			errs = append(errs, fmt.Errorf("%s: uuid is required", label))
		case seen[f.UUID]:
			errs = append(errs, fmt.Errorf("%s: duplicate uuid %q", label, f.UUID))
		}
		seen[f.UUID] = true

		for _, err := range f.problems() {
			errs = append(errs, fmt.Errorf("%s: %w", label, err))
		}
	}
	return errors.Join(errs...)
}
