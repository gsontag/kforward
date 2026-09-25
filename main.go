// Kforward is a tray indicator to start and stop kubectl-like port-forwards.
package main

import (
	"flag"
	"os"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func main() {
	check := flag.Bool("check", false, "check the configuration against the kubeconfig and exit")
	flag.Parse()
	if *check {
		os.Exit(runCheck())
	}
	app := gtk.NewApplication("fr.gsontag.kforward", gio.ApplicationDefaultFlags)
	app.ConnectActivate(func() { activate(app) })

	// GApplication parses command line too and would reject our options
	if code := app.Run(append([]string{os.Args[0]}, flag.Args()...)); code > 0 {
		os.Exit(code)
	}
}

func activate(app *gtk.Application) {
	window := gtk.NewApplicationWindow(app)
	window.SetTitle("gotk4 Example")
	window.SetChild(gtk.NewLabel("Hello from Go!"))
	window.SetDefaultSize(400, 300)
	window.SetVisible(true)
}
