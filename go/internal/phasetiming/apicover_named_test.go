package phasetiming

// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported phasetiming symbol by identifier (apicover counts
// field access as "uses", not "names"). Each test asserts a REAL contract.

import (
	"strings"
	"testing"
)

// TestSummary_RollupNamed names the Summary type and Rollup, pinning the roll-up
// contract that the latency evidence turns on: total = sum of durations, longest
// = max, retried = dispatches past attempt 1.
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
	// ArchetypePercent on an empty Summary must not divide by zero.
	if p := (Summary{}).ArchetypePercent("build"); p != 0 {
		t.Errorf("empty Summary ArchetypePercent=%.1f, want 0", p)
	}
}

// TestPath_FileNameNamed names Path + FileName: the timing-log path is the
// workspace joined with the canonical file name.
func TestPath_FileNameNamed(t *testing.T) {
	t.Parallel()
	if got := Path("/ws"); !strings.HasSuffix(got, FileName) {
		t.Errorf("Path(/ws)=%q must end with FileName %q", got, FileName)
	}
}

// TestHumanMS_Named names HumanMS: millisecond durations render compactly, and
// sub-second values round to "0s" (the deliberate second-resolution choice).
func TestHumanMS_Named(t *testing.T) {
	t.Parallel()
	if got := HumanMS(60_000); got != "1m0s" {
		t.Errorf("HumanMS(60000)=%q, want 1m0s", got)
	}
	if got := HumanMS(0); got != "0s" {
		t.Errorf("HumanMS(0)=%q, want 0s", got)
	}
}
