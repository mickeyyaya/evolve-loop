package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/attemptpostmortem"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLocateTranscript_FindsTheSessionOfASeatThatRanOnTheToken(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	dir := filepath.Join(home, ".evolve-seat", "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"user","cwd":"/wt","timestamp":"2026-10-09T10:00:00Z","message":{"content":"x"}}` + "\n"
	want := filepath.Join(dir, "seat.jsonl")
	if err := os.WriteFile(want, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(Options{Hooks: &fakeHooks{phase: "build"}})
	w := launchWindow{cli: "claude-tmux", start: mustStamp(t, "2026-10-09T09:59:00Z"), end: mustStamp(t, "2026-10-09T10:01:00Z")}
	s := postmortemScope{req: core.PhaseRequest{Worktree: "/wt", Env: map[string]string{"HOME": home}}}
	if got := r.locateTranscript(s, w, attemptpostmortem.FormatClaudeJSONL); got != want {
		t.Fatalf("the post-mortem must read a seat session under the seat root: got %q, want %q", got, want)
	}
}
