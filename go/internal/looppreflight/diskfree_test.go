package looppreflight

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiskFreeBytes_MeasuresTheFilesystemHoldingThePath(t *testing.T) {
	free, err := DiskFreeBytes(t.TempDir())
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		if err == nil {
			t.Fatal("an unsupported platform must report the probe as unsupported, never a zero that reads as a full disk")
		}
		return
	}
	if err != nil || free == 0 {
		t.Fatalf("DiskFreeBytes = %d, %v; want the temp filesystem's free bytes", free, err)
	}
	if _, err := DiskFreeBytes(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing path must be an error, not a measurement")
	}
}
