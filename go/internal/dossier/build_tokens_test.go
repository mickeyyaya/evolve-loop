package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// TestBuild_ProjectsPhaseTokens — S6, token-telemetry. Build must project each
// timed phase's terminal token usage (Entry.Tokens, S4) onto the durable
// PhaseRecord, so the committed dossier records per-phase token counts beside
// duration — not just dollars.
func TestBuild_ProjectsPhaseTokens(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	entries := []phasetiming.Entry{
		{Phase: "scout", DurationMS: 400_000, Verdict: "PASS", Archetype: "plan", AttemptCount: 1,
			Tokens: cyclestate.TokenUsage{Input: 600, Output: 90, CacheRead: 300, CacheWrite: 20}},
		{Phase: "build", DurationMS: 700_000, Verdict: "PASS", Archetype: "build", AttemptCount: 1,
			Tokens: cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 80, CacheWrite: 16}},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Build(9, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	byName := map[string]PhaseRecord{}
	for _, p := range d.Phases {
		byName[p.Name] = p
	}
	want := cyclestate.TokenUsage{Input: 1200, Output: 340, CacheRead: 80, CacheWrite: 16}
	if byName["build"].Tokens != want {
		t.Fatalf("build PhaseRecord.Tokens = %+v, want %+v", byName["build"].Tokens, want)
	}
	if byName["scout"].Tokens.Input != 600 {
		t.Fatalf("scout PhaseRecord.Tokens.Input = %d, want 600", byName["scout"].Tokens.Input)
	}
}
