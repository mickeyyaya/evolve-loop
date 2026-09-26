package tokenusage

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
	want := cyclestate.TokenUsage{Input: 10, Output: 9, CacheRead: 2, CacheWrite: 0}
	if res.Usage != want {
		t.Errorf("Usage = %+v, want %+v (dedup by message id — must take the last delta, not sum all three)", res.Usage, want)
	}
}

func TestTranscriptScan_ConcurrentSessionsSameDir_OnlyContentVerifiedCounted(t *testing.T) {
	worktree := "/repo/worktrees/cycle-997"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "projects", "-repo-worktrees-cycle-997")

	bodyA := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"uA","content":[{"type":"text","text":"Artifact path: .evolve/runs/cycle-997/launch-token-abc123"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:02Z","message":{"id":"mA","usage":{"input_tokens":40,"output_tokens":4,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	// Session B shares the cwd and session directory but not the launch's artifact path.
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

func TestTranscriptScan_MissingDirYieldsSourceNone(t *testing.T) {
	root := t.TempDir()
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
