//go:build !darwin && !linux

package looppreflight

import "errors"

func defaultDiskUsageOf(string) (DiskUsage, error) {
	return DiskUsage{}, errors.New("disk-free probe unsupported on this platform")
}

func defaultDiskFreeBytes(path string) (uint64, error) {
	_, err := defaultDiskUsageOf(path)
	return 0, err
}
