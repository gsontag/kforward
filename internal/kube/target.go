package kube

import (
	"fmt"
	"strings"
)

// Kind is a kind of Kubernetes object that can be port-forwarded to.
type Kind string

// Supported target kinds.
const (
	KindPod         Kind = "pod"
	KindService     Kind = "service"
	KindDeployment  Kind = "deployment"
	KindStatefulSet Kind = "statefulset"
)

// Same names and shortnames as kubectl
var kindAliases = map[string]Kind{
	"pod": KindPod, "pods": KindPod, "po": KindPod,
	"service": KindService, "services": KindService, "svc": KindService,
	"deployment": KindDeployment, "deployments": KindDeployment, "deploy": KindDeployment,
	"statefulset": KindStatefulSet, "statefulsets": KindStatefulSet, "sts": KindStatefulSet,
}

// Target is a parsed forward target, such as svc/grafana
type Target struct {
	Kind Kind
	Name string
}

// ParseTarget parses a kubectl-style target: KIND/NAME, or NAME alone for a pod.
func ParseTarget(s string) (Target, error) {
	kind, name, found := strings.Cut(s, "/")
	if !found {
		kind, name = string(KindPod), s
	}
	k, ok := kindAliases[strings.ToLower(kind)]
	if !ok {
		return Target{}, fmt.Errorf("target %q: unsupported kind %q", s, kind)
	}
	if name == "" || strings.Contains(name, "/") {
		return Target{}, fmt.Errorf("target %q: expected KIND/NAME", s)
	}
	return Target{Kind: k, Name: name}, nil
}

func (t Target) String() string {
	return string(t.Kind) + "/" + t.Name
}
