package logfile

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// read returns the content of path, "<absent>" when there is no such file.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "<absent>"
	}
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

// appendLine opens path like the application does and writes a line.
func appendLine(t *testing.T, path string, maxSize int64, line string) {
	t.Helper()
	f, err := Open(path, maxSize)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOpenCreatesTheDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "kforward", "kforward.log")

	appendLine(t, path, 100, "first")

	check(t, "log", read(t, path), "first\n")
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// The log tells which clusters and pods the user works with
	check(t, "dir mode", info.Mode().Perm(), os.FileMode(0o700))
	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	check(t, "file mode", info.Mode().Perm(), os.FileMode(0o600))
}

func TestOpenAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kforward.log")

	appendLine(t, path, 100, "first")
	appendLine(t, path, 100, "second")

	check(t, "log", read(t, path), "first\nsecond\n")
	check(t, "rotated", read(t, path+".1"), "<absent>")
}

func TestOpenRotates(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantLog     string
		wantRotated string
	}{
		{"under the limit", "12345678\n", "12345678\nnew\n", "<absent>"},
		// Rotated only when larger than the limit, not when equal
		{"at the limit", "123456789", "123456789new\n", "<absent>"},
		{"over the limit", "1234567890", "new\n", "1234567890"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "kforward.log")
			write(t, path, tt.content)

			appendLine(t, path, 9, "new")

			check(t, "log", read(t, path), tt.wantLog)
			check(t, "rotated", read(t, path+".1"), tt.wantRotated)
		})
	}
}

func TestOpenReplacesTheOldRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kforward.log")
	write(t, path+".1", "oldest")
	write(t, path, "older, too long")

	appendLine(t, path, 9, "new")

	// Only one old log is kept: the size stays bounded
	check(t, "log", read(t, path), "new\n")
	check(t, "rotated", read(t, path+".1"), "older, too long")
}

func TestOpenErrors(t *testing.T) {
	dir := t.TempDir()
	// A file where the directory should be
	blocker := filepath.Join(dir, "file")
	write(t, blocker, "")

	if f, err := Open(filepath.Join(blocker, "kforward.log"), 100); err == nil {
		_ = f.Close()
		t.Error("Open under a file: no error")
	}
	// A directory where the log should be; large limit, or it would be rotated
	if f, err := Open(dir, 1<<20); err == nil {
		_ = f.Close()
		t.Error("Open on a directory: no error")
	}
}

func check[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
