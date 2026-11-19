//go:build darwin || linux

package looppreflight

import (
	"syscall"
	"testing"
)

func TestDefaultDiskFreeBytes_RealPath(t *testing.T) {
	bytes, err := defaultDiskFreeBytes(t.TempDir())
	if err != nil {
		t.Fatalf("defaultDiskFreeBytes: %v", err)
	}
	if bytes == 0 {
		t.Fatal("expected non-zero free bytes")
	}
}

func TestDiskUsageFromStatfs_ValidAndNonPositiveBsize(t *testing.T) {
	if _, err := diskUsageFromStatfs(syscall.Statfs_t{Bsize: 0}); err == nil {
		t.Fatal("diskUsageFromStatfs with Bsize=0 = nil, want error")
	}
	u, err := diskUsageFromStatfs(syscall.Statfs_t{Bsize: 4096, Bavail: 10, Blocks: 100})
	if err != nil {
		t.Fatalf("diskUsageFromStatfs valid = %v, want nil", err)
	}
	if u.FreeBytes != 40960 || u.TotalBytes != 409600 {
		t.Fatalf("diskUsageFromStatfs = %+v, want FreeBytes=40960 TotalBytes=409600", u)
	}
}
