package tokenusage

// inputcache_fidelity_test.go — cycle-779 TDD contract for the
// token-telemetry-input-cache-fidelity task (inbox weight 0.96,
// operator-boosted 2026-07-13). The 2026-07-13 live baseline showed
// input=0/cache_read=0/cache_write=0 across every phase: only OUTPUT tokens
// survive to `evolve tokens report`, hiding the dominant cost dimension
// (input outweighs output 2:1–100:1 per
// knowledge-base/research/token-optimization-2026).
//
// This file pins the AC1 claude-transcript half of the fix: usage blocks in
// the claude CLI transcript JSONL carry input_tokens /
// cache_read_input_tokens / cache_creation_input_tokens, and the scanner must
// surface ALL of them — not just output — with cache-dominated magnitudes
// (the realistic shape: cache_read >> input).
//
// The per-driver coverage half (agy/codex fail-open + WARN counters) needs a
// new seam (Window carries no driver today) and is bound by name in
// go/acs/cycle779/predicates_test.go: TestScanner_PerDriverCoverageWarnsNotZeros,
// TestScanner_UnknownDriverFailsOpenNoError (Builder authors test+seam; the
// ACS predicates stay RED until they exist and pass).

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// TestScanner_ExtractsInputAndCacheFromClaudeUsageBlocks: a realistic
// cache-dominated claude transcript (cache_read ~25x input, input ~2x output)
// must yield non-zero Input/CacheRead/CacheWrite — the exact fields the
// 2026-07-13 baseline reported as all-zero. An output-only extraction fails
// three of the four field assertions.
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

// TestScanner_UsageBlockMissingCacheFieldsStaysZeroNotFabricated (edge): a
// usage block that omits the cache fields (older CLI format) must contribute
// zero cache counts — the scanner must not fabricate cache data it cannot
// observe, while still extracting the input/output it can.
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
