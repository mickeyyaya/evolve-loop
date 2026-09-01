//go:build darwin || linux

package reportdoc

import (
	"os"
	"syscall"
)

// OpenRegularNoFollow opens path read-only without following a final-component
// symlink (O_NOFOLLOW). Shared belief for every bounded read of an
// agent-writable file — single home; do not copy this helper into another
// package (architecture review 2026-09-01).
func OpenRegularNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
