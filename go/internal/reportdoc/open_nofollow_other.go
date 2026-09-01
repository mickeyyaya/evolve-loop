//go:build !darwin && !linux

package reportdoc

import "os"

// OpenRegularNoFollow on platforms without O_NOFOLLOW support falls back to a
// plain open, which DOES follow symlinks; every caller must pair it with its
// own Lstat/SameFile identity checks (all current callers do).
func OpenRegularNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
