//go:build !darwin && !linux

package looppreflight

import "errors"

// defaultDiskFreeBytes is unsupported here; the host check skips the disk warning on error.
func defaultDiskFreeBytes(string) (uint64, error) {
	return 0, errors.New("disk-free probe unsupported on this platform")
}
