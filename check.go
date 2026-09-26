package main

import (
	"context"
	"fmt"
	"os"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
)

// runCheck validates the configuration against the kubeconfig, without
// starting any forward. It returns the process exit code.
func runCheck() int {
	path, store, client, err := loadAll()
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
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	cluster, endpoint, err := resolveForward(ctx, client, f)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"[%s] %s/%s → pod %s:%d, listening on %s:%d",
		cluster.Context,
		endpoint.Namespace,
		f.Target,
		endpoint.Pod,
		endpoint.Port,
		f.BindAddress(),
		f.LocalPort,
	), nil
}
