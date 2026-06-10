//go:build darwin || linux

package looppreflight

import "testing"

func TestDefaultDiskFreeBytes_RealPath(t *testing.T) {
	bytes, err := defaultDiskFreeBytes(t.TempDir())
	if err != nil {
		t.Fatalf("defaultDiskFreeBytes: %v", err)
	}
	if bytes == 0 {
		t.Fatal("expected non-zero free bytes")
	}
}
