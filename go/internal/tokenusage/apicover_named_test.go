package tokenusage

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestResultAndSourceNamed(t *testing.T) {
	var s Source = SourceTranscript
	if s == SourceNone {
		t.Fatal("SourceTranscript and SourceNone must be distinct")
	}
	r := Result{Source: SourceNone}
	if r.Source != SourceNone || r.Usage != (cyclestate.TokenUsage{}) {
		t.Fatalf("zero Result must carry SourceNone, got %q", r.Source)
	}
	if string(SourceNone) != "none" || string(SourceTranscript) != "transcript" {
		t.Fatalf("Source string values drifted: none=%q transcript=%q", SourceNone, SourceTranscript)
	}
}

func TestResultPeakPromptTokensNamed(t *testing.T) {
	r := Result{
		Usage:            cyclestate.TokenUsage{Input: 10, CacheRead: 90},
		Source:           SourceTranscript,
		PeakPromptTokens: 60,
	}
	if got := windowOccupancy(r); got != 60 {
		t.Fatalf("windowOccupancy = %d, want 60 (Result.PeakPromptTokens, not the summed Usage's 100)", got)
	}
	if got := windowOccupancy(Result{Usage: r.Usage, Source: SourceEventsResult}); got != 100 {
		t.Fatalf("windowOccupancy = %d, want 100 — a tier with no per-turn breakdown falls back to its whole-launch total", got)
	}
}

func TestResultPeakUsageNamed(t *testing.T) {
	worktree := "/repo/worktrees/cycle-peak"
	root := t.TempDir()
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"go"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":200,"output_tokens":7,"cache_read_input_tokens":6800,"cache_creation_input_tokens":0}}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:09Z","message":{"id":"m2","usage":{"input_tokens":1000,"output_tokens":9,"cache_read_input_tokens":1000,"cache_creation_input_tokens":0}}}
`
	writeTranscript(t, filepath.Join(root, "projects", "p"), "s.jsonl", body)
	r, err := ScanConfigRoot(root, Window{Worktree: worktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)})
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	want := cyclestate.TokenUsage{Input: 200, Output: 7, CacheRead: 6_800}
	if r.PeakUsage != want || r.PeakPromptTokens != 7_000 {
		t.Fatalf("PeakUsage = %+v peak = %d, want %+v and 7000 (the largest single prompt, not the sum)", r.PeakUsage, r.PeakPromptTokens, want)
	}
}

func TestCollectorTypeNamed(t *testing.T) {
	var c Collector = func() Result {
		return Result{Usage: cyclestate.TokenUsage{Output: 3}, Source: SourceScrollbackPeak}
	}
	if got := Chain(c); got.Source != SourceScrollbackPeak || got.Usage.Output != 3 {
		t.Fatalf("Collector literal not run through the chain: got %+v", got)
	}
}
