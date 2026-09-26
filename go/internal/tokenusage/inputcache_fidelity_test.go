package tokenusage

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks(t *testing.T) {
	worktree := "/repo/worktrees/cycle-779-fidelity"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-779-fidelity")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":1200,"output_tokens":600,"cache_read_input_tokens":30000,"cache_creation_input_tokens":4500}}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:09Z","message":{"id":"m2","usage":{"input_tokens":800,"output_tokens":400,"cache_read_input_tokens":20000,"cache_creation_input_tokens":500}}}
`
	writeTranscript(t, sessionDir, "sess-fidelity.jsonl", body)

	w := Window{
		Worktree: worktree,
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	if res.Source != SourceTranscript {
		t.Fatalf("Source = %q, want %q", res.Source, SourceTranscript)
	}
	want := cyclestate.TokenUsage{Input: 2000, Output: 1000, CacheRead: 50000, CacheWrite: 5000}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v (output-only extraction leaves Input/CacheRead/CacheWrite zero — the 2026-07-13 baseline defect)", res.Usage, want)
	}
}

func TestScanner_UsageBlockMissingCacheFieldsStaysZeroNotFabricated(t *testing.T) {
	worktree := "/repo/worktrees/cycle-779-nocache"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-779-nocache")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":700,"output_tokens":300}}}
`
	writeTranscript(t, sessionDir, "sess-nocache.jsonl", body)

	w := Window{
		Worktree: worktree,
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	want := cyclestate.TokenUsage{Input: 700, Output: 300, CacheRead: 0, CacheWrite: 0}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v (absent cache fields must stay zero, not be fabricated)", res.Usage, want)
	}
}
