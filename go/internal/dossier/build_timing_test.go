package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

// When a phase-timing.json exists in the workspace, Build must ingest it: one
// PhaseRecord per timed phase carrying duration/start/end/archetype, plus a
// cycle-level Timing summary — so the committed dossier (the durable, git-tracked
// evidence) records WHERE the cycle spent its wall-clock, not just the verdict.
func TestBuild_IngestsPhaseTiming(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	entries := []phasetiming.Entry{
		{Phase: "scout", DurationMS: 400_000, Verdict: "PASS", Archetype: "plan", AttemptCount: 1, StartedAt: "2026-06-25T00:00:00Z", EndedAt: "2026-06-25T00:06:40Z"},
		{Phase: "build", DurationMS: 700_000, Verdict: "PASS", Archetype: "build", AttemptCount: 1},
		{Phase: "audit", DurationMS: 300_000, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Build(7, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Real per-phase records replace the stub.
	if len(d.Phases) != 3 {
		t.Fatalf("want 3 phase records from the timing log, got %d: %+v", len(d.Phases), d.Phases)
	}
	byName := map[string]PhaseRecord{}
	for _, p := range d.Phases {
		byName[p.Name] = p
	}
	if byName["build"].DurationMS != 700_000 {
		t.Errorf("build PhaseRecord DurationMS=%d, want 700000", byName["build"].DurationMS)
	}
	if byName["scout"].Archetype != "plan" || byName["scout"].StartedAt == "" {
		t.Errorf("scout record must carry archetype+start: %+v", byName["scout"])
	}

	// Cycle-level roll-up.
	if d.Timing == nil {
		t.Fatal("Dossier.Timing summary must be populated when a timing log exists")
	}
	if d.Timing.LongestPhase != "build" {
		t.Errorf("Timing.LongestPhase=%q, want build", d.Timing.LongestPhase)
	}
	if d.Timing.TotalMS != 1_400_000 {
		t.Errorf("Timing.TotalMS=%d, want 1400000", d.Timing.TotalMS)
	}
}

// The markdown timing section must render deterministically: text/template
// iterates maps in sorted key order, so the archetype rows are stable across
// renders. This locks that contract (a non-deterministic render would corrupt
// committed dossiers and break diffs).
func TestRenderMarkdown_TimingSectionDeterministic(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	entries := []phasetiming.Entry{
		{Phase: "scout", DurationMS: 100, Verdict: "PASS", Archetype: "plan", AttemptCount: 1},
		{Phase: "build", DurationMS: 200, Verdict: "PASS", Archetype: "build", AttemptCount: 1},
		{Phase: "audit", DurationMS: 150, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
		{Phase: "ship", DurationMS: 5, Verdict: "PASS", Archetype: "control", AttemptCount: 1},
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Build(9, BuildOpts{WorkspacePath: ws, Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	first, err := RenderMarkdown(d)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	for i := 0; i < 8; i++ {
		again, err := RenderMarkdown(d)
		if err != nil {
			t.Fatalf("RenderMarkdown (rerun): %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("markdown render is non-deterministic across calls:\n--- first ---\n%s\n--- run %d ---\n%s", first, i, again)
		}
	}
}

// No timing log → Build keeps the always-valid stub (backward compatible).
func TestBuild_NoTimingLogKeepsStub(t *testing.T) {
	t.Parallel()
	d, err := Build(8, BuildOpts{WorkspacePath: t.TempDir(), Goal: "g", FinalVerdict: VerdictPass})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(d.Phases) == 0 {
		t.Error("Build must still produce >=1 phase record without a timing log")
	}
	if d.Timing != nil {
		t.Errorf("Timing must be nil without a timing log; got %+v", d.Timing)
	}
}
