package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newEntry(t *testing.T, exec string) Entry {
	t.Helper()
	// A directory to create: the autostart one may not exist yet
	return Entry{
		Dir:  filepath.Join(t.TempDir(), "autostart"),
		ID:   "fr.gsontag.kforward",
		Name: "Kube Forwarder",
		Exec: exec,
	}
}

func read(t *testing.T, e Entry) string {
	t.Helper()
	b, err := os.ReadFile(e.path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

func TestEnableDisable(t *testing.T) {
	e := newEntry(t, "/home/me/.local/bin/kforward")
	check(t, "enabled at first", e.Enabled(), false)

	if err := e.Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	check(t, "enabled", e.Enabled(), true)
	want := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Kube Forwarder\n" +
		"Exec=/home/me/.local/bin/kforward --background\n" +
		"Icon=fr.gsontag.kforward\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
	check(t, "content", read(t, e), want)
	check(t, "file", filepath.Base(e.path()), "fr.gsontag.kforward.desktop")

	if err := e.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	check(t, "enabled after disable", e.Enabled(), false)
	if _, err := os.Stat(e.path()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("entry still there after Disable: %v", err)
	}
	// Twice, or without an entry at all: nothing to do
	if err := e.Disable(); err != nil {
		t.Errorf("second Disable: %v", err)
	}
}

// A desktop may turn the entry off in place: it counts as disabled, and
// Enable turns it on again.
func TestTurnedOffByTheDesktop(t *testing.T) {
	for _, line := range []string{"Hidden=true", "X-GNOME-Autostart-enabled=false"} {
		t.Run(line, func(t *testing.T) {
			e := newEntry(t, "/usr/bin/kforward")
			if err := e.Enable(); err != nil {
				t.Fatalf("Enable: %v", err)
			}
			content := strings.Replace(read(t, e), "X-GNOME-Autostart-enabled=true\n", line+"\n", 1)
			if err := os.WriteFile(e.path(), []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			check(t, "enabled when turned off", e.Enabled(), false)

			if err := e.Enable(); err != nil {
				t.Fatalf("Enable: %v", err)
			}
			check(t, "enabled again", e.Enabled(), true)
		})
	}
}

func TestEnableRefusesPercent(t *testing.T) {
	e := newEntry(t, "/opt/100%/kforward")

	if err := e.Enable(); !errors.Is(err, ErrPercent) {
		t.Errorf("Enable = %v, want ErrPercent", err)
	}
	// No entry GLib could not start
	check(t, "enabled", e.Enabled(), false)
}

func TestEnableError(t *testing.T) {
	e := newEntry(t, "/usr/bin/kforward")
	// A file where the autostart directory should be
	if err := os.WriteFile(e.Dir, nil, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.Enable(); err == nil {
		t.Error("Enable under a file: no error")
	}
}

// The expected values were checked by launching the entries with gio launch,
// the launcher of GLib that GNOME uses at login.
func TestQuoteExec(t *testing.T) {
	tests := []struct {
		name, path, want string
	}{
		{"plain", "/home/me/.local/bin/kforward", "/home/me/.local/bin/kforward"},
		{"space", "/home/me/My Apps/kforward", `"/home/me/My Apps/kforward"`},
		{"single quote", "/home/me/l'app/kforward", `"/home/me/l'app/kforward"`},
		{"reserved", "/opt/a;b(c)/kforward", `"/opt/a;b(c)/kforward"`},
		// Escaped for the quotes, then each backslash doubled for the string
		{"double quote", `/opt/a"b/kforward`, `"/opt/a\\"b/kforward"`},
		{"dollar", "/opt/a$b/kforward", `"/opt/a\\$b/kforward"`},
		{"backquote", "/opt/a`b/kforward", "\"/opt/a\\\\`b/kforward\""},
		{"backslash", `/opt/a\b/kforward`, `"/opt/a\\\\b/kforward"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check(t, "quoted", quoteExec(tt.path), tt.want)
		})
	}
}

func TestDefault(t *testing.T) {
	e, err := Default("fr.gsontag.kforward", "Kube Forwarder")
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	// The running program: here the test binary
	check(t, "exec", e.Exec, exe)
	check(t, "dir", filepath.Base(e.Dir), "autostart")
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
