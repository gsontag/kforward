package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gsontag.fr/kforward/internal/config"
	"gsontag.fr/kforward/internal/forward"
)

// runForward runs the named forward in the foreground, reconnecting it when
// it drops, until interrupted. It returns the process exit code.
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

	connector, err := forward.NewClusterConnector(client, f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", f.Name, err)
		return 1
	}

	// Cancelled by Ctrl-C or kill: stops the forward and its reconnections
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var final forward.Status
	forward.Run(ctx, connector, forward.DefaultPolicy, func(s forward.Status) {
		final = s
		switch {
		case s.State == forward.Active:
			fmt.Printf(
				"%s: active, %s:%d → pod %s/%s:%d\n",
				f.Name,
				f.BindAddress(),
				f.LocalPort,
				s.Endpoint.Namespace,
				s.Endpoint.Pod,
				s.Endpoint.Port,
			)
		case s.Err != nil:
			fmt.Printf("%s: %s (%v)\n", f.Name, s.State, s.Err)
		default:
			fmt.Printf("%s: %s\n", f.Name, s.State)
		}
	})
	if final.State == forward.Failed {
		return 1
	}
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
