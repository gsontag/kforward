package main

import (
	"time"

	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/kube"
)

// resolveTimeout bounds the wait when a cluster does not answer or an auth
// plugin hangs.
const resolveTimeout = 10 * time.Second

// loadAll loads the configuration and the kubeconfig it points to.
func loadAll() (string, *config.Store, *kube.Client, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return "", nil, nil, err
	}
	store := config.NewStore(path)
	if err := store.Load(); err != nil {
		return "", nil, nil, err
	}
	client, err := kube.NewClient(store.Kubeconfig())
	if err != nil {
		return "", nil, nil, err
	}
	return path, store, client, nil
}
