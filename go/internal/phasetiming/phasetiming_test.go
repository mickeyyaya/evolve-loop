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

// ProjectParallelSaving is the shadow evidence for PR2: it projects how much
// wall-clock the independent post-build evaluate phases (archetype "evaluate",
// excluding the audit brancher) would save if run concurrently. Pure projection
// from recorded durations — no execution change.
func TestProjectParallelSaving(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Phase: "scout", DurationMS: 400_000, Archetype: "plan"},  // not evaluate — excluded
		{Phase: "build", DurationMS: 700_000, Archetype: "build"}, // not evaluate — excluded
		{Phase: "coverage-gate", DurationMS: 240_000, Archetype: "evaluate"},
		{Phase: "behavior-compare", DurationMS: 120_000, Archetype: "evaluate"},
		{Phase: "adversarial-review", DurationMS: 300_000, Archetype: "evaluate"},
		{Phase: "audit", DurationMS: 290_000, Archetype: "evaluate"}, // the brancher — excluded
	}
	// Group = the 3 non-audit evaluate phases: 240+120+300 = 660s sequential.
	// At concurrency 2, makespan lower bound = max(maxDur 300, ceil(660/2)=330) = 330s.
	p := ProjectParallelSaving(entries, 2)
	if p.Concurrency != 2 {
		t.Errorf("Concurrency=%d, want 2", p.Concurrency)
	}
	if len(p.GroupPhases) != 3 {
		t.Errorf("GroupPhases=%v, want 3 non-audit evaluate phases", p.GroupPhases)
	}
	if p.SequentialMS != 660_000 {
		t.Errorf("SequentialMS=%d, want 660000", p.SequentialMS)
	}
	if p.ProjectedParallelMS != 330_000 {
		t.Errorf("ProjectedParallelMS=%d, want 330000 (makespan lower bound)", p.ProjectedParallelMS)
	}
	if p.SavingMS != 330_000 {
		t.Errorf("SavingMS=%d, want 330000", p.SavingMS)
	}
}

// Concurrency >= group size ⇒ makespan = the single longest phase.
func TestProjectParallelSaving_UnboundedConcurrency(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{Phase: "a", DurationMS: 100, Archetype: "evaluate"},
		{Phase: "b", DurationMS: 500, Archetype: "evaluate"},
		{Phase: "c", DurationMS: 200, Archetype: "evaluate"},
	}
	p := ProjectParallelSaving(entries, 8)
	if p.ProjectedParallelMS != 500 {
		t.Errorf("ProjectedParallelMS=%d, want 500 (the longest phase)", p.ProjectedParallelMS)
	}
	if p.SavingMS != 300 { // 800 sequential - 500 makespan
		t.Errorf("SavingMS=%d, want 300", p.SavingMS)
	}
}

// Degenerate cases: <2 parallelizable phases, or concurrency<=1, ⇒ zero saving.
func TestProjectParallelSaving_NoParallelism(t *testing.T) {
	t.Parallel()
	one := []Entry{{Phase: "audit", DurationMS: 300, Archetype: "evaluate"}} // only audit
	if p := ProjectParallelSaving(one, 4); p.SavingMS != 0 || len(p.GroupPhases) != 0 {
		t.Errorf("single audit phase: want zero saving / empty group, got %+v", p)
	}
	two := []Entry{
		{Phase: "a", DurationMS: 100, Archetype: "evaluate"},
		{Phase: "b", DurationMS: 200, Archetype: "evaluate"},
	}
	if p := ProjectParallelSaving(two, 1); p.SavingMS != 0 {
		t.Errorf("concurrency 1: want zero saving, got %+v", p)
	}
}
