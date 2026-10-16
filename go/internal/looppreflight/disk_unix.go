//go:build darwin || linux

package looppreflight

import (
	"errors"
	"syscall"
)

func defaultDiskFreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	if st.Bsize <= 0 {
		return 0, errors.New("statfs: non-positive block size")
	}
	return st.Bavail * uint64(st.Bsize), nil
}
