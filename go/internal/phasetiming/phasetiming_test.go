package phasetiming

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAndRollup(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	entries := []Entry{
		{Phase: "scout", DurationMS: 400_000, Verdict: "PASS", Archetype: "plan", AttemptCount: 1, StartedAt: "2026-06-25T00:00:00Z", EndedAt: "2026-06-25T00:06:40Z"},
		{Phase: "build", DurationMS: 700_000, Verdict: "PASS", Archetype: "build", AttemptCount: 2},
		{Phase: "audit", DurationMS: 300_000, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
		{Phase: "ship", DurationMS: 2_000, Verdict: "PASS", Archetype: "control", AttemptCount: 1},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Read(ws)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("Read returned %d entries, want 4", len(got))
	}

	s := Rollup(got)
	if s.TotalMS != 1_402_000 {
		t.Errorf("TotalMS=%d, want 1402000", s.TotalMS)
	}
	if s.PhaseCount != 4 {
		t.Errorf("PhaseCount=%d, want 4", s.PhaseCount)
	}
	if s.RetriedCount != 1 {
		t.Errorf("RetriedCount=%d, want 1 (build took 2 attempts)", s.RetriedCount)
	}
	if s.LongestPhase != "build" || s.LongestMS != 700_000 {
		t.Errorf("Longest=%s/%d, want build/700000", s.LongestPhase, s.LongestMS)
	}
	if s.ByArchetype["build"] != 700_000 || s.ByArchetype["evaluate"] != 300_000 {
		t.Errorf("ByArchetype=%v, want build=700000 evaluate=300000", s.ByArchetype)
	}
	// build is 700000/1402000 ≈ 49.9%
	if p := s.ArchetypePercent("build"); p < 49 || p > 51 {
		t.Errorf("build percent=%.1f, want ~49.9", p)
	}
}

// Legacy logs written before the archetype field existed must still sum (under
// the "unknown" bucket) rather than be dropped from the total.
func TestRollup_LegacyEntriesBucketUnknown(t *testing.T) {
	t.Parallel()
	s := Rollup([]Entry{{Phase: "scout", DurationMS: 100, AttemptCount: 1}})
	if s.TotalMS != 100 {
		t.Errorf("TotalMS=%d, want 100", s.TotalMS)
	}
	if s.ByArchetype["unknown"] != 100 {
		t.Errorf("archetype-less entry must bucket under 'unknown'; got %v", s.ByArchetype)
	}
}

func TestRead_MissingFileIsNotExist(t *testing.T) {
	t.Parallel()
	_, err := Read(t.TempDir())
	if !os.IsNotExist(err) {
		t.Errorf("missing file must return an os.IsNotExist error; got %v", err)
	}
}
