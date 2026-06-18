package looppreflight

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultDirWritable reports whether dir can be created and written to, using
// the same mkdir → touch sentinel → remove probe as preflight.probeWritable
// (which is unexported there). Best-effort: any failure means "not writable".
func defaultDirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe := filepath.Join(dir, fmt.Sprintf(".looppreflight-probe.%d", os.Getpid()))
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}
