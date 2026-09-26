package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/kube"
)

// runForward runs the named forward in the foreground until interrupted.
// It returns the process exit code.
func runForward(name string) int {
	_, store, client, err := loadAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	f, ok := findForward(store.Forwards(), name)
	if !ok {
		fmt.Fprintf(os.Stderr, "no forward named %q\n", name)
		return 1
	}

	// Cancelled by Ctrl-C or kill: stops the resolution as well as the forward
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resolveCtx, cancel := context.WithTimeout(ctx, resolveTimeout)
	cluster, endpoint, err := resolveForward(resolveCtx, client, f)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", f.Name, err)
		return 1
	}

	err = kube.Forward(ctx, cluster, endpoint, f.BindAddress(), f.LocalPort, func() {
		fmt.Printf(
			"%s: %s:%d → pod %s/%s:%d (Ctrl-C to stop)\n",
			f.Name,
			f.BindAddress(),
			f.LocalPort,
			endpoint.Namespace,
			endpoint.Pod,
			endpoint.Port,
		)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", f.Name, err)
		return 1
	}
	fmt.Printf("%s: stopped\n", f.Name)
	return 0
}

func findForward(forwards []config.Forward, name string) (config.Forward, bool) {
	for _, f := range forwards {
		if f.Name == name {
			return f, true
		}
	}
	return config.Forward{}, false
}
