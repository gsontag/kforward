package main

import (
	"fmt"
	"os"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
)

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
		cluster, err := client.Cluster(f.Context)
		if err != nil {
			fmt.Printf("  x %s: %v\n", f.Name, err)
			status = 1
			continue
		}
		namespace := f.Namespace
		if namespace == "" {
			namespace = cluster.Namespace
		}
		fmt.Printf(
			"  ✓ %s: [%s] %s/%s:%s → %s:%d\n",
			f.Name,
			cluster.Context,
			namespace,
			f.Target,
			f.RemotePort.String(),
			f.BindAddress(),
			f.LocalPort,
		)
	}
	return status
}
