// Package kube gives access to the Kubernetes clusters of the kubeconfig.
package kube

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Cluster is a kubeconfig context resolved into a usable REST configuration.
type Cluster struct {
	Context string
	// Namespace is the default namespace of the context, "default" when unset.
	Namespace string
	// Config is shared between callers and must not be modified
	Config *rest.Config
}

type contextEntry struct {
	cluster *Cluster
	err     error
}

// Client is an immutable snapshot of the kubeconfig: reloading means building
// a new one. It is safe for concurrent use.
type Client struct {
	current  string
	contexts map[string]contextEntry
}

// NewClient loads the kubeconfig. An empty path applies the kubectl loading
// rules: the KUBECONFIG files, merged, then ~/.kube/config. A broken context
// does not fail the whole load: its error is returned by Cluster.
func NewClient(kubeconfigPath string) (*Client, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = kubeconfigPath

	raw, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	contexts := make(map[string]contextEntry, len(raw.Contexts))
	for name := range raw.Contexts {
		contexts[name] = resolveContext(raw, name, rules)
	}
	return &Client{current: raw.CurrentContext, contexts: contexts}, nil
}

func resolveContext(
	raw *api.Config,
	name string,
	rules *clientcmd.ClientConfigLoadingRules,
) contextEntry {
	// Without this check, client-go reports an obscure KUBERNETES_MASTER error.
	if cluster := raw.Contexts[name].Cluster; raw.Clusters[cluster] == nil {
		return contextEntry{err: fmt.Errorf("context %q: cluster %q not found", name, cluster)}
	}

	cc := clientcmd.NewNonInteractiveClientConfig(*raw, name, &clientcmd.ConfigOverrides{}, rules)
	restConfig, err := cc.ClientConfig()
	if err != nil {
		return contextEntry{err: fmt.Errorf("context %q: %w", name, err)}
	}
	namespace, _, err := cc.Namespace()
	if err != nil {
		return contextEntry{err: fmt.Errorf("context %q: %w", name, err)}
	}
	return contextEntry{cluster: &Cluster{Context: name, Namespace: namespace, Config: restConfig}}
}

// Contexts returns the names of the kubeconfig contexts, sorted.
func (c *Client) Contexts() []string {
	return slices.Sorted(maps.Keys(c.contexts))
}

// CurrentContext returns the current-context of the kubeconfig, possibly empty
func (c *Client) CurrentContext() string {
	return c.current
}

// Cluster returns the cluster of the named context, or of the current context
// when name is empty
func (c *Client) Cluster(name string) (*Cluster, error) {
	if name == "" {
		if c.current == "" {
			return nil, errors.New("no context given and no current-context in kubeconfig")
		}
		name = c.current
	}
	entry, ok := c.contexts[name]
	if !ok {
		return nil, fmt.Errorf("unknown context %q", name)
	}
	return entry.cluster, entry.err
}
