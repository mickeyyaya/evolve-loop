//go:build !darwin && !linux

package looppreflight

import "errors"

func defaultDiskFreeBytes(string) (uint64, error) {
	return 0, errors.New("disk-free probe unsupported on this platform")
}
