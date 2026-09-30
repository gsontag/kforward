package gtk

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestFileURI(t *testing.T) {
	check(t, "plain", FileURI("/home/me/.config/kforward/config.json"),
		"file:///home/me/.config/kforward/config.json")
	// Escaped: an editor could not open the file otherwise
	check(t, "space", FileURI("/home/me/My Configs/config.json"),
		"file:///home/me/My%20Configs/config.json")
}

func TestLaunchUnknownScheme(t *testing.T) {
	if err := LaunchDefaultForURI("kforward-no-such-scheme://x"); err == nil {
		t.Error("no application for the scheme: got no error")
	}
}

// The store saves by writing a temporary file, then renaming it over the
// configuration: the monitor must see the rename.
func TestFileMonitorSeesARenameOver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var events []FileMonitorEvent
	var err error
	onMain(t, func() {
		var m *FileMonitor
		m, err = MonitorFile(path)
		if err == nil {
			m.ConnectChanged(func(e FileMonitorEvent) { events = append(events, e) })
			// Kept until the end of the test, or the monitor stops
			t.Cleanup(func() { _ = m })
		}
	})
	if err != nil {
		t.Fatalf("MonitorFile: %v", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(`{"forwards": []}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		var seen bool
		onMain(t, func() {
			iterate()
			seen = slices.Contains(events, FileMonitorEventRenamed) ||
				slices.Contains(events, FileMonitorEventMovedIn)
		})
		if seen {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no rename seen, events: %v", events)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
