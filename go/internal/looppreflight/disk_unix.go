//go:build darwin || linux

package looppreflight

import (
	"errors"
	"syscall"
)

func defaultDiskUsageOf(path string) (DiskUsage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskUsage{}, err
	}
	return diskUsageFromStatfs(st)
}

func diskUsageFromStatfs(st syscall.Statfs_t) (DiskUsage, error) {
	if st.Bsize <= 0 {
		return DiskUsage{}, errors.New("statfs: non-positive block size")
	}
	bsize := uint64(st.Bsize)
	return DiskUsage{
		FreeBytes:  st.Bavail * bsize,
		TotalBytes: st.Blocks * bsize,
	}, nil
}

func defaultDiskFreeBytes(path string) (uint64, error) {
	u, err := defaultDiskUsageOf(path)
	if err != nil {
		return 0, err
	}
	return u.FreeBytes, nil
}
