package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
)

const checkTimeout = 10 * time.Second

// runCheck validates the configuration against the kubeconfig, without
// starting any forward. It returns the process exit code.
func runCheck() int {
	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	store := config.NewStore(path)
	if err := store.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	client, err := kube.NewClient(store.Kubeconfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("config:   %s\n", path)
	fmt.Printf("contexts: %v (current: %q)\n", client.Contexts(), client.CurrentContext())

	status := 0
	for _, f := range store.Forwards() {
		line, err := checkForward(client, f)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", f.Name, err)
			status = 1
			continue
		}
		fmt.Printf("  ✓ %s: %s\n", f.Name, line)
	}
	return status
}

// checkForward resolves the pod and port a forward would connect to.
func checkForward(client *kube.Client, f config.Forward) (string, error) {
	cluster, err := client.Cluster(f.Context)
	if err != nil {
		return "", err
	}
	target, err := kube.ParseTarget(f.Target)
	if err != nil {
		return "", err
	}
	namespace := f.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}

	// Bounds the wait when the cluster does not answer or an auth plugin hangs
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	endpoint, err := kube.Resolve(ctx, cluster.Clientset, namespace, target, f.RemotePort)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"[%s] %s/%s → pod %s:%d, listening on %s:%d",
		cluster.Context,
		namespace,
		f.Target,
		endpoint.Pod,
		endpoint.Port,
		f.BindAddress(),
		f.LocalPort,
	), nil
}
