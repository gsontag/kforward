package main

import (
	"log/slog"
	"os"

	"github.com/adrg/xdg"
	"k8s.io/klog/v2"

	"gsontag.fr/kforward/internal/logfile"
)

// maxLogSize bounds the log file: a tray application runs for weeks.
const maxLogSize = 1 << 20

// setupLog sends the log of the application, client-go's included, to the
// log file and to stderr. It returns the path of the file, empty when it
// could not be opened: the log then goes to stderr only.
func setupLog() string {
	stderr := slog.NewTextHandler(os.Stderr, nil)
	handler := slog.Handler(stderr)

	path, err := xdg.StateFile("kforward/kforward.log")
	if err == nil {
		var file *os.File
		if file, err = logfile.Open(path, maxLogSize); err == nil {
			// Never closed: the log is written until the process exits
			handler = slog.NewMultiHandler(slog.NewTextHandler(file, nil), stderr)
		}
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	// client-go logs through klog: its reconnection errors belong to the history
	klog.SetSlogLogger(logger.With("source", "client-go"))

	if err != nil {
		slog.Warn("log file", "err", err)
		return ""
	}
	return path
}
