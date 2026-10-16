package budgethistory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func writeRunWithTokens(t *testing.T, root string, cycle int, durationMS int64, tokens []cyclestate.TokenUsage) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir run ws: %v", err)
	}
	entries := make([]phasetiming.Entry, 0, len(tokens))
	for i, tok := range tokens {
		entries = append(entries, phasetiming.Entry{
			Phase:      fmt.Sprintf("phase%d", i),
			DurationMS: durationMS / int64(len(tokens)),
			Verdict:    "PASS",
			Tokens:     tok,
		})
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal timing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatalf("write timing: %v", err)
	}
}

func TestCollect_MedianTokensPerCycle(t *testing.T) {
	root := t.TempDir()
	writeRunWithTokens(t, root, 20, 600_000, []cyclestate.TokenUsage{
		{Input: 100, Output: 10, CacheRead: 0, CacheWrite: 0},
		{Input: 100, Output: 10, CacheRead: 0, CacheWrite: 0},
	})
	writeRunWithTokens(t, root, 21, 600_000, []cyclestate.TokenUsage{
		{Input: 100, Output: 10, CacheRead: 1000, CacheWrite: 0},
	})
	writeRunWithTokens(t, root, 22, 600_000, []cyclestate.TokenUsage{
		{Input: 200, Output: 30, CacheRead: 2000, CacheWrite: 170},
	})

	got := Collect(root, []int{20, 21, 22})

	if got.SampleCount != 3 {
		t.Fatalf("SampleCount = %d, want 3", got.SampleCount)
	}
	if got.MedianTokensPerCycle != 1110 {
		t.Errorf("MedianTokensPerCycle = %d, want 1110 (gross median of 220/1110/2400; "+
			"220 would mean cache tokens were dropped)", got.MedianTokensPerCycle)
	}
	if got.MedianCycleDurationMS != 600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 600000 (duration path unchanged)", got.MedianCycleDurationMS)
	}
}

func TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian(t *testing.T) {
	root := t.TempDir()
	writeRun(t, root, 30, oneHour(), 0)
	writeRun(t, root, 31, oneHour(), 0)

	got := Collect(root, []int{30, 31})

	if got.SampleCount != 2 {
		t.Fatalf("SampleCount = %d, want 2 (legacy cycles still carry duration)", got.SampleCount)
	}
	if got.MedianTokensPerCycle != 0 {
		t.Errorf("MedianTokensPerCycle = %d, want 0 (legacy log ⇒ absent, not fabricated)", got.MedianTokensPerCycle)
	}
	if got.MedianCycleDurationMS != 3_600_000 {
		t.Errorf("MedianCycleDurationMS = %d, want 3600000", got.MedianCycleDurationMS)
	}
}

func TestCollect_NoEvidenceYieldsZeroTokenMedian(t *testing.T) {
	got := Collect(t.TempDir(), []int{99})

	if got.SampleCount != 0 || got.MedianTokensPerCycle != 0 {
		t.Errorf("Collect(no evidence) = %+v, want zero-value Throughput", got)
	}
}
