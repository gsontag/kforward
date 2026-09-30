package gtk

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// calls are run by TestMain on the main thread, the only one allowed to use
// GTK; the tests run in goroutines of their own.
var calls = make(chan func())

// gtkReady tells whether GTK could be initialized, on a display of its own.
var gtkReady bool

func TestMain(m *testing.M) {
	stop := startDisplay()
	gtkReady = initGTK()

	code := make(chan int)
	go func() { code <- m.Run() }()
	for {
		select {
		case f := <-calls:
			f()
		case c := <-code:
			stop()
			os.Exit(c)
		}
	}
}

// startDisplay starts a Broadway display for the tests, so that they never
// open windows on the desktop, and work without one, as in CI.
func startDisplay() (stop func()) {
	display := fmt.Sprintf(":%d", 100+os.Getpid()%100)
	dir, err := os.MkdirTemp("", "kforward-gtk")
	if err != nil {
		return func() {}
	}
	cmd := exec.Command("gtk4-broadwayd", "--address", "127.0.0.1", display)
	// Its socket goes to XDG_RUNTIME_DIR: a private one, removed at the end
	cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR="+dir)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return func() {}
	}
	socket := filepath.Join(dir, "broadway"+display[1:]+".socket")
	for i := 0; i < 50 && !exists(socket); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	for k, v := range map[string]string{
		"GDK_BACKEND": "broadway", "BROADWAY_DISPLAY": display, "XDG_RUNTIME_DIR": dir,
	} {
		_ = os.Setenv(k, v)
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = os.RemoveAll(dir)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// onMain runs f on the main thread, and waits for it. The test is skipped
// if GTK could not be initialized: gtk4-broadwayd is missing. f must not
// call t.Fatal, which only works from the goroutine of the test.
func onMain(t *testing.T, f func()) {
	t.Helper()
	if !gtkReady {
		t.Skip("GTK not initialized: is gtk4-broadwayd installed?")
	}
	done := make(chan struct{})
	calls <- func() {
		defer close(done)
		f()
	}
	<-done
}
