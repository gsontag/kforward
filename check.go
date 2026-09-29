package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gsontag/kforward/internal/config"
	"github.com/gsontag/kforward/internal/forward"
	"github.com/gsontag/kforward/internal/kube"
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
			fmt.Printf("  KO %s: %v\n", f.Name, err)
			status = 1
			continue
		}
		fmt.Printf("  OK %s: %s\n", f.Name, line)
	}
	return status
}

// checkForward resolves the pod and port a forward would connect to.
func checkForward(client *kube.Client, f config.Forward) (string, error) {
	connector, err := forward.NewClusterConnector(client, f)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	endpoint, err := connector.Resolve(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"[%s] %s/%s -> pod %s:%d, listening on %s:%d",
		connector.Context(),
		endpoint.Namespace,
		f.Target,
		endpoint.Pod,
		endpoint.Port,
		f.BindAddress(),
		f.LocalPort,
	), nil
}
