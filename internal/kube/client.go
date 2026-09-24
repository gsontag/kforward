package kube

import (
	"fmt"

	"gsontag.fr/kforward/internal/config"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type KubeForward struct {
	Forward  *config.Forward
	IsActive bool
}

type Client struct {
	config     *config.Store
	kubeConfig *api.Config
	forwards   map[string][]KubeForward
}

func NewKubeClient(configStore *config.Store) (*Client, error) {
	config, err := clientcmd.LoadFromFile(configStore.Kubeconfig())
	if err != nil {
		return nil, fmt.Errorf("error loading kubeconfig: %w", err)
	}

	fws := configStore.Forwards()
	forwards := make(map[string][]KubeForward, 0)

	for _, fw := range fws {
		if group := forwards[fw.Group]; group != nil {
			forwards[fw.Group] = append(group, KubeForward{
				Forward:  &fw,
				IsActive: false,
			})
		} else {
			forwards[fw.Group] = []KubeForward{
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

func (c *Client) GetForwards() map[string][]KubeForward {
	return c.forwards
}

func (c *Client) GetContexts() []string {
	keys := make([]string, 0, len(c.kubeConfig.Contexts))
	for k := range c.kubeConfig.Contexts {
		keys = append(keys, k)
	}

	return keys
}
