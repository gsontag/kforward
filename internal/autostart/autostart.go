// Package autostart starts the application at login, through the XDG
// autostart directory that every desktop reads.
package autostart

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// ErrPercent refuses a program whose path contains %: GLib, which starts
// the entries on GNOME, launches such a path under no escaping.
var ErrPercent = errors.New("the path of the program contains %")

// Entry is the autostart file of an application.
type Entry struct {
	// Dir is the autostart directory
	Dir string
	// ID is the application ID: it names the file and the icon
	ID   string
	Name string
	// Exec is the program to start
	Exec string
}

// Default returns the entry of the running program, in the autostart
// directory of the user.
func Default(id, name string) (Entry, error) {
	exec, err := os.Executable()
	if err != nil {
		return Entry{}, fmt.Errorf("find the program: %w", err)
	}
	return Entry{
		Dir:  filepath.Join(xdg.ConfigHome, "autostart"),
		ID:   id,
		Name: name,
		Exec: exec,
	}, nil
}

func (e Entry) path() string {
	return filepath.Join(e.Dir, e.ID+".desktop")
}

// Enabled tells whether the application starts at login. A desktop may hide
// an entry rather than remove it, when the user turns it off in its settings.
func (e Entry) Enabled() bool {
	content, err := os.ReadFile(e.path())
	if err != nil {
		return false
	}
	for line := range strings.Lines(string(content)) {
		switch strings.TrimSpace(line) {
		case "Hidden=true", "X-GNOME-Autostart-enabled=false":
			return false
		}
	}
	return true
}

// Enable writes the entry, replacing one the user turned off.
func (e Entry) Enable() error {
	if strings.Contains(e.Exec, "%") {
		return fmt.Errorf("%w: %s", ErrPercent, e.Exec)
	}
	if err := os.MkdirAll(e.Dir, 0o700); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}
	// Without StartupNotify: no window comes, the desktop would wait for one
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=" + e.Name + "\n" +
		"Exec=" + quoteExec(e.Exec) + " --background\n" +
		"Icon=" + e.ID + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"
	if err := os.WriteFile(e.path(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write autostart entry: %w", err)
	}
	return nil
}

// Disable removes the entry; it is not an error if there is none.
func (e Entry) Disable() error {
	if err := os.Remove(e.path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove autostart entry: %w", err)
	}
	return nil
}

// quoteExec writes path as the program of an Exec key. The desktop entry
// specification wants quotes around a path with reserved characters, and a
// backslash before ", `, $ and \ inside them.
func quoteExec(path string) string {
	if !strings.ContainsAny(path, " \t\"'\\><~|&;$*?#()`") {
		return path
	}
	quoted := `"` + strings.NewReplacer(`"`, `\"`, "`", "\\`", `$`, `\$`, `\`, `\\`).
		Replace(path) +
		`"`
	// The value is a string too, whose own escaping doubles every backslash
	return strings.ReplaceAll(quoted, `\`, `\\`)
}
