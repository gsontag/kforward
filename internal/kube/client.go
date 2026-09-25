// Package kube gives access to the Kubernetes clusters of the kubeconfig.
package kube

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"gsontag.fr/kforward/internal/config"
)

// LiveForward pairs a configured forward with its runtime state.
type LiveForward struct {
	Forward  *config.Forward
	IsActive bool
}

// Client is the access layer to the Kubernetes clusters.
type Client struct {
	config     *config.Store
	kubeConfig *api.Config
	forwards   map[string][]LiveForward
}

// NewKubeClient loads the kubeconfig and indexes the forwards by group.
func NewKubeClient(configStore *config.Store) (*Client, error) {
	config, err := clientcmd.LoadFromFile(configStore.Kubeconfig())
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %w", err)
	}

	fws := configStore.Forwards()
	forwards := make(map[string][]LiveForward, 0)

	for _, fw := range fws {
		if group := forwards[fw.Group]; group != nil {
			forwards[fw.Group] = append(group, LiveForward{
				Forward:  &fw,
				IsActive: false,
			})
		} else {
			forwards[fw.Group] = []LiveForward{
				{
					Forward:  &fw,
					IsActive: false,
				},
			}
		}
	}

	// TODO: activate autostart

	return &Client{
		config:     configStore,
		kubeConfig: config,
		forwards:   forwards,
	}, nil
}

// GetForwards returns the forwards, indexed by group.
func (c *Client) GetForwards() map[string][]LiveForward {
	return c.forwards
}

// GetContexts returns the names of the contexts defined in the kubeconfig.
func (c *Client) GetContexts() []string {
	keys := make([]string, 0, len(c.kubeConfig.Contexts))
	for k := range c.kubeConfig.Contexts {
		keys = append(keys, k)
	}

	return keys
}
