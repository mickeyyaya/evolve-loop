package phasetiming

import (
	"strings"
	"testing"
)

func TestParallelProjection_Named(t *testing.T) {
	t.Parallel()
	var p ParallelProjection = ProjectParallelSaving([]Entry{
		{Phase: "coverage-gate", DurationMS: 400, Archetype: "evaluate"},
		{Phase: "behavior-compare", DurationMS: 200, Archetype: "evaluate"},
	}, 2)
	if p.SavingMS == 0 || len(p.GroupPhases) != 2 {
		t.Errorf("expected a positive saving over 2 evaluate phases at C=2; got %+v", p)
	}
}

func TestSummary_RollupNamed(t *testing.T) {
	t.Parallel()
	var s Summary = Rollup([]Entry{
		{Phase: "scout", DurationMS: 400, Archetype: "plan", AttemptCount: 1},
		{Phase: "build", DurationMS: 700, Archetype: "build", AttemptCount: 2},
	})
	if s.TotalMS != 1100 {
		t.Errorf("Summary.TotalMS=%d, want 1100", s.TotalMS)
	}
	if s.LongestPhase != "build" || s.LongestMS != 700 {
		t.Errorf("Summary longest=%s/%d, want build/700", s.LongestPhase, s.LongestMS)
	}
	if s.RetriedCount != 1 {
		t.Errorf("Summary.RetriedCount=%d, want 1", s.RetriedCount)
	}
	if p := (Summary{}).ArchetypePercent("build"); p != 0 {
		t.Errorf("empty Summary ArchetypePercent=%.1f, want 0", p)
	}
}

func TestPath_FileNameNamed(t *testing.T) {
	t.Parallel()
	if got := Path("/ws"); !strings.HasSuffix(got, FileName) {
		t.Errorf("Path(/ws)=%q must end with FileName %q", got, FileName)
	}
}

func TestHumanMS_Named(t *testing.T) {
	t.Parallel()
	if got := HumanMS(60_000); got != "1m0s" {
		t.Errorf("HumanMS(60000)=%q, want 1m0s", got)
	}
	if got := HumanMS(0); got != "0s" {
		t.Errorf("HumanMS(0)=%q, want 0s", got)
	}
}
