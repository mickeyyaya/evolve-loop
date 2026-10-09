package attemptpostmortem

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	fixtureAttempt1   = "testdata/cycle1853-build-attempt1.jsonl"
	fixtureAttempt2   = "testdata/cycle1853-build-attempt2.jsonl"
	fixtureFinalPane1 = "testdata/cycle1853-build-attempt1-final-pane.txt"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func cycle1853BuildAttempt1(t *testing.T) Attempt {
	t.Helper()
	return Attempt{
		Phase:      "build",
		Cycle:      1853,
		Number:     1,
		CLI:        "claude-tmux",
		Session:    "evolve-bridge-r01M4FY77-c1853-build-pid64934-n6-1791537560",
		DispatchID: "01M4FY77W5XZNZGJ1NSMN06N3J/1853/build/p64934n6",
		StartedAt:  mustTime(t, "2026-10-09T09:19:20.147269Z"),
		EndedAt:    mustTime(t, "2026-10-09T09:59:52.11564Z"),
		CauseCode:  "review_pause",
		ExitCode:   81,
	}
}

func collectAttempt1(t *testing.T, cfg Config) Record {
	t.Helper()
	rec, err := Collect(Input{
		Attempt:        cycle1853BuildAttempt1(t),
		Transcript:     TranscriptFor("claude-tmux", fixtureAttempt1),
		ScrollbackPath: fixtureFinalPane1,
		WorktreeDelta:  " go/internal/swarm/dispatcher_test.go | 31 ++++++++++++++++++++++++-------\n 1 file changed, 24 insertions(+), 7 deletions(-)",
	}, cfg)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return rec
}

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
