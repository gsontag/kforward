// Package logfile keeps the log of the application in a file of bounded
// size, where the user can look for what happened.
package logfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Open opens the log file at path for appending. A file larger than maxSize
// is rotated first: its content moves to path.1, replacing an older one, so
// that the log never takes more than twice maxSize.
func Open(path string, maxSize int64) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	info, err := os.Stat(path)
	switch {
	case err == nil && info.Size() > maxSize:
		if err := os.Rename(path, path+".1"); err != nil {
			return nil, fmt.Errorf("rotate log: %w", err)
		}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("stat log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	return f, nil
}
