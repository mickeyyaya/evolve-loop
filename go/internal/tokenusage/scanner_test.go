package tokenusage

// scanner_test.go — RED contract for token-telemetry S1 (docs/plans/
// token-telemetry-2026-07.md S1; inbox token-telemetry-s1-transcript-scanner,
// weight 0.95). internal/tokenusage does not exist yet: every symbol below
// (Window, Result, Source*, ScanConfigRoot) is undefined, so this package
// fails to compile — the intended RED signal. Builder implements the scanner
// against this contract; do not modify these tests.
//
// Fixture shape mirrors the real Claude Code transcript JSONL: one JSON
// object per line, "type" in {"user","assistant"}, a top-level "cwd", and for
// assistant lines a "message" object carrying "id" (message id, repeated
// across streamed deltas for the same logical turn) and "usage" (token
// counts). ScanConfigRoot must sum usage across all assistant lines whose cwd
// matches the launch Window's Worktree exactly (never trust the session
// directory name/slug), deduplicating repeated message ids (streamed usage
// deltas), and tie-breaking multiple same-cwd session files by requiring the
// Window's ArtifactPath to appear in the first user message's text.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func writeTranscript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

const launchWindowStart = "2026-07-07T10:00:00Z"
const launchWindowEnd = "2026-07-07T10:10:00Z"

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return ts
}

// TestTranscriptScan_SumsUsageWithinWindow: two assistant turns inside the
// launch window, both cwd-matched to the worktree, sum their usage fields
// into cyclestate.TokenUsage and report SourceTranscript.
func TestTranscriptScan_SumsUsageWithinWindow(t *testing.T) {
	worktree := "/repo/worktrees/cycle-999"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-999")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":5,"cache_creation_input_tokens":1}}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:09Z","message":{"id":"m2","usage":{"input_tokens":50,"output_tokens":10,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	w := Window{
		Worktree: worktree,
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	want := cyclestate.TokenUsage{Input: 150, Output: 30, CacheRead: 5, CacheWrite: 1}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v", res.Usage, want)
	}
	if res.Source != SourceTranscript {
		t.Errorf("Source = %q, want %q", res.Source, SourceTranscript)
	}
}

// TestTranscriptScan_DeduplicatesStreamedUsageByMessageID: the CLI streams
// usage deltas for the SAME logical turn under one message id; only the last
// (highest-cumulative) line per id counts, never a raw sum of every line.
func TestTranscriptScan_DeduplicatesStreamedUsageByMessageID(t *testing.T) {
	worktree := "/repo/worktrees/cycle-998"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-998")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:02Z","message":{"id":"m1","usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:03Z","message":{"id":"m1","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:04Z","message":{"id":"m1","usage":{"input_tokens":10,"output_tokens":9,"cache_read_input_tokens":2,"cache_creation_input_tokens":0}}}
`
	writeTranscript(t, sessionDir, "sess1.jsonl", body)

	w := Window{
		Worktree: worktree,
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	// Only the final streamed delta for message id "m1" must count.
	want := cyclestate.TokenUsage{Input: 10, Output: 9, CacheRead: 2, CacheWrite: 0}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v (dedup by message id — must take the last delta, not sum all three)", res.Usage, want)
	}
}

// TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted:
// swarm lanes can share one session-directory slug (the slug is a lossy
// sanitization of cwd, not a unique key). When two transcript files in the
// SAME session directory both claim the matching cwd, only the one whose
// first user message contains the launch's unique ArtifactPath is counted —
// the other must be excluded even though its cwd also matches.
func TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted(t *testing.T) {
	worktree := "/repo/worktrees/cycle-997"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-997")

	// Session A: the real launch — first user message cites the unique
	// artifact path the orchestrator stamped for this launch.
	bodyA := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"uA","content":[{"type":"text","text":"working in .evolve/runs/cycle-997/launch-token-abc123"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:02Z","message":{"id":"mA","usage":{"input_tokens":40,"output_tokens":4,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	// Session B: a concurrent swarm reader sharing the same sanitized dir and
	// (coincidentally) the same cwd, but its first user message does NOT cite
	// this launch's artifact path — must be excluded.
	bodyB := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"uB","content":[{"type":"text","text":"unrelated swarm reader task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:03Z","message":{"id":"mB","usage":{"input_tokens":9000,"output_tokens":9000,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	writeTranscript(t, sessionDir, "sessA.jsonl", bodyA)
	writeTranscript(t, sessionDir, "sessB.jsonl", bodyB)

	w := Window{
		Worktree:     worktree,
		ArtifactPath: ".evolve/runs/cycle-997/launch-token-abc123",
		Start:        mustParse(t, launchWindowStart),
		End:          mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot: %v", err)
	}
	want := cyclestate.TokenUsage{Input: 40, Output: 4}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v (only the content-verified session — cwd match alone must not be trusted)", res.Usage, want)
	}
	if res.Source != SourceTranscript {
		t.Errorf("Source = %q, want %q — a clean content-verified single match is unambiguous", res.Source, SourceTranscript)
	}
}

// TestTranscriptScan_MissingDirYieldsSourceNone: no projects/<slug> directory
// under the config root at all (e.g. tmux driver, or CLI never wrote a
// transcript) must yield a zero Usage and SourceNone, never an error — token
// telemetry is best-effort instrumentation, not a hard requirement.
func TestTranscriptScan_MissingDirYieldsSourceNone(t *testing.T) {
	root := t.TempDir() // no projects/ subdirectory created at all
	w := Window{
		Worktree: "/repo/worktrees/cycle-996",
		Start:    mustParse(t, launchWindowStart),
		End:      mustParse(t, launchWindowEnd),
	}
	res, err := ScanConfigRoot(root, w)
	if err != nil {
		t.Fatalf("ScanConfigRoot on missing dir must not error: %v", err)
	}
	if res.Usage != (cyclestate.TokenUsage{}) {
		t.Errorf("Usage = %+v, want zero value", res.Usage)
	}
	if res.Source != SourceNone {
		t.Errorf("Source = %q, want %q", res.Source, SourceNone)
	}
}
