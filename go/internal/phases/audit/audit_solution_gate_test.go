package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// A document cycle whose solutions/<slug>/ fails the deterministic contract
// (internal/solutioncheck) must FAIL audit even when the narrative says PASS
// and EGPS is green — the same single-exit gate shape as gofmt (ADR-0099 slice 2).
func TestRun_SolutionContractViolation_FAILsAudit(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:  &fakeBridge{writeArtifact: body},
		Prompts: fakePromptsFS("body"),
		CheckSolution: func(core.PhaseRequest) ([]string, error) {
			return []string{"solutions/netflix-margin/options: found 1 option file(s), need at least 2"}, nil
		},
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Fatalf("Verdict=%q, want FAIL (solution contract violated)", resp.Verdict)
	}
	if !hasDiagContaining(resp.Diagnostics, "solutions/netflix-margin") {
		t.Errorf("want a diagnostic naming the violation; got %+v", resp.Diagnostics)
	}
}

// A clean deliverable (or a code cycle: no failures) keeps PASS; an infra
// error fails OPEN with a warning.
func TestRun_SolutionContractClean_PASSPreserved(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	body := "# Audit Report\n\n## Verdict\n**PASS**\n"
	phase := New(Config{
		Bridge:        &fakeBridge{writeArtifact: body},
		Prompts:       fakePromptsFS("body"),
		CheckSolution: func(core.PhaseRequest) ([]string, error) { return nil, nil },
	})
	if resp, _ := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws}); resp.Verdict != core.VerdictPASS {
		t.Fatalf("Verdict=%q, want PASS", resp.Verdict)
	}
}

// TestNew_SolutionSpecWiresTheGate: the composition root hands the registry
// spec in; a document cycle bound to no task then FAILs through the production
// gate (no config loading inside the phase).
func TestNew_SolutionSpecWiresTheGate(t *testing.T) {
	ws := t.TempDir()
	writeACSVerdict(t, ws, 0)
	for name, body := range map[string]string{
		"triage-report.md":     "<!-- challenge-token: x -->\n# Triage\n\ncycle_size_estimate: medium\ndeliverable_kind: document\n\n## top_n\n",
		"triage-decision.json": `{"top_n":[]}`,
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	spec := config.DeliverableKindSpec{Root: "solutions", MinOptions: 2}
	phase := New(Config{
		Bridge:       &fakeBridge{writeArtifact: "# Audit Report\n\n## Verdict\n**PASS**\n"},
		Prompts:      fakePromptsFS("body"),
		SolutionSpec: &spec,
	})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{Cycle: 1, ProjectRoot: "/p", Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictFAIL || !hasDiagContaining(resp.Diagnostics, "binds no task") {
		t.Fatalf("verdict=%q diags=%+v; want FAIL naming the empty binding", resp.Verdict, resp.Diagnostics)
	}
}

// TestNewDefaultWithStageCompactSpec_WiresTheGate: the composition root's
// constructor threads the registry spec into the production gate.
func TestNewDefaultWithStageCompactSpec_WiresTheGate(t *testing.T) {
	spec := config.DeliverableKindSpec{Root: "solutions", MinOptions: 2}
	if phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: "x"}, fakePromptsFS("body"), config.StageOff, false, &spec); phase == nil || phase.BaseRunner == nil {
		t.Fatal("constructor must return a wired phase")
	}
	if phase := NewDefaultWithStageCompactSpec(&fakeBridge{writeArtifact: "x"}, fakePromptsFS("body"), config.StageOff, false, nil); phase == nil {
		t.Fatal("nil spec ⇒ phase without a solution gate, never a nil phase")
	}
}
