package gc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestDiscover_APollutedArchiveIsADeadRunWithoutAMarker(t *testing.T) {
	dir := t.TempDir()
	runs := filepath.Join(dir, "runs")
	archive := filepath.Join(runs, "cycle-1604"+gcpolicy.PollutedArchiveName(time.Date(2026, 9, 1, 21, 14, 50, 11361000, time.UTC)))
	writeFile(t, filepath.Join(archive, "gc-shadow-manifest.json"), "{}")
	lookalike := filepath.Join(runs, "notes.polluted-20260901T211450.011361000")
	if err := os.MkdirAll(lookalike, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(dir, DiscoverOptions{})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	byPath := map[string]RunDir{}
	for _, r := range got {
		byPath[r.Path] = r
	}
	if r, ok := byPath[archive]; !ok || r.Live {
		t.Errorf("polluted archive: discovered=%v live=%v, want a dead run that the ladder can age", ok, r.Live)
	}
	if _, ok := byPath[lookalike]; ok {
		t.Errorf("a markerless dir whose name only looks like an archive was discovered: %s", lookalike)
	}
}
