package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReapPipelineTemp_ReapsStaleSuiteTempDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	stale := now.Add(-30 * time.Hour)
	names := []string{"TestBuildShip1234567890", "TestGC_Journey2718281828", "gittest3141592653"}
	for _, name := range names {
		p := filepath.Join(dir, name)
		writeFile(t, filepath.Join(p, "001", "payload"), "0123456789")
		if err := os.Chtimes(p, stale, stale); err != nil {
			t.Fatal(err)
		}
	}

	rep := ReapPipelineTemp(dir, now.Add(-24*time.Hour), os.RemoveAll)

	if rep.Entries != len(names) {
		t.Errorf("reaped %d entries, want %d", rep.Entries, len(names))
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s: stale suite temp dir survived the reaper", name)
		}
	}
}
