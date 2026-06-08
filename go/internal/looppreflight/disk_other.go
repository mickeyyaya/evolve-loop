//go:build !darwin && !linux

package looppreflight

import "errors"

// defaultDiskFreeBytes is unsupported off darwin/linux; the host-capabilities
// check treats the error as "skip the disk warning" (the evolve-loop runtime
// targets darwin/linux only).
func defaultDiskFreeBytes(string) (uint64, error) {
	return 0, errors.New("disk-free probe unsupported on this platform")
}
