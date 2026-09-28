// Kforward is a tray indicator to start and stop kubectl-like port-forwards.
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func main() {
	check := flag.Bool("check", false, "check the configuration against the kubeconfig and exit")
	forward := flag.String(
		"forward",
		"",
		"run the named forward in the foreground until interrupted",
	)
	flag.Parse()
	switch {
	case *check:
		os.Exit(runCheck())
	case *forward != "":
		os.Exit(runForward(*forward))
	}
	application := gtk.NewApplication("fr.gsontag.kforward", gio.ApplicationDefaultFlags)
	a := &app{gtk: application}
	application.ConnectActivate(a.activate)
	application.ConnectShutdown(a.shutdown)

	// Ctrl-C or kill: quit through GTK, so that shutdown stops the forwards
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		post(application.Quit)
	}()

	// GApplication parses command line too and would reject our options
	if code := application.Run(append([]string{os.Args[0]}, flag.Args()...)); code > 0 {
		os.Exit(code)
	}
}
