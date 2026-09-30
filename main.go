// Kforward is a tray indicator to start and stop kubectl-like port-forwards.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gsontag/kforward/internal/gtk"
)

// appID names the application for GTK and the desktop: the .desktop file and
// the icon in data/ are named after it.
const appID = "fr.gsontag.kforward"

// appName is the name the desktop shows, as in the .desktop file.
const appName = "Kube Forwarder"

func main() {
	check := flag.Bool("check", false, "check the configuration against the kubeconfig and exit")
	forward := flag.String(
		"forward",
		"",
		"run the named forward in the foreground until interrupted",
	)
	background := flag.Bool("background", false, "start in the tray, without opening the window")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	switch {
	case *showVersion:
		fmt.Println("kforward", version())
		return
	case *check:
		os.Exit(runCheck())
	case *forward != "":
		os.Exit(runForward(*forward))
	}
	application := gtk.NewApplication(appID)
	a := &app{gtk: application, background: *background}
	application.ConnectActivate(a.activate)
	application.ConnectShutdown(a.shutdown)

	// Ctrl-C or kill: quit through GTK, so that shutdown stops the forwards
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		post(application.Quit)
	}()

	// Registering early tells whether an instance already runs: started in
	// the background, there is nothing to bring to the front
	if err := application.Register(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *background && application.IsRemote() {
		return
	}

	// GApplication parses command line too and would reject our options
	if code := application.Run(append([]string{os.Args[0]}, flag.Args()...)); code > 0 {
		os.Exit(code)
	}
}
