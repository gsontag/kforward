package main

import (
	"context"
	"time"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
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

// resolveForward finds the cluster, pod and port a configured forward points to.
func resolveForward(
	ctx context.Context,
	client *kube.Client,
	f config.Forward,
) (*kube.Cluster, kube.Endpoint, error) {
	cluster, err := client.Cluster(f.Context)
	if err != nil {
		return nil, kube.Endpoint{}, err
	}
	target, err := kube.ParseTarget(f.Target)
	if err != nil {
		return nil, kube.Endpoint{}, err
	}
	namespace := f.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}
	endpoint, err := kube.Resolve(ctx, cluster.Clientset, namespace, target, f.RemotePort)
	return cluster, endpoint, err
}
