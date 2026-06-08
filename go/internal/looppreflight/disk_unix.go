//go:build darwin || linux

package looppreflight

import (
	"errors"
	"syscall"
)

// defaultDiskFreeBytes returns the bytes available to an unprivileged user on
// the filesystem holding path, via statfs. Bavail is uint64 on both platforms;
// Bsize is int64 on linux and uint32 on darwin, so the <=0 guard (which fail-
// loud rejects a nonsense block size rather than letting an unchecked cast wrap
// it into a huge "ample disk" value) covers both.
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
