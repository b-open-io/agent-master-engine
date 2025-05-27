//go:build !windows
// +build !windows

package daemon

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

// LockFile provides file-based locking using flock
type LockFile struct {
	path string
	file *os.File
}

// NewLockFile creates a new lock file instance
func NewLockFile(path string) *LockFile {
	return &LockFile{path: path}
}

// TryAcquire attempts to acquire an exclusive lock
func (l *LockFile) TryAcquire() error {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK {
			// Try to read PID from file
			if data, err := os.ReadFile(l.path); err == nil && len(data) > 0 {
				return fmt.Errorf("lock held by process %s", string(data))
			}
			return fmt.Errorf("lock held by another process")
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Write our PID
	file.Truncate(0)
	file.WriteString(strconv.Itoa(os.Getpid()))
	file.Sync()

	l.file = file
	return nil
}

// Release releases the lock
func (l *LockFile) Release() error {
	if l.file == nil {
		return nil
	}

	// Close file (releases flock automatically)
	err := l.file.Close()
	l.file = nil

	// Remove lock file
	os.Remove(l.path)

	return err
}