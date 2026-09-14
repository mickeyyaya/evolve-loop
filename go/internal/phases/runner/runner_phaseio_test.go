package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestRun_DefaultProbeHonorsPhaseIO (ADR-0050 Phase 3.10 Slice 1): with no
// VerifyFn injected, the verdict engine judges with the catalog-aware probe
// New resolves over Options.PhaseIO — at enforce a build FAIL-without-block
// report is caught (parity with the host gate: the ship guard turns the hook's
// PASS into a coherent FAIL and the violation is on the response) and below
// enforce it stays dormant (byte-identical: PASS). Pinned THROUGH Run since
// review fold F4 — the probe lives in the engine only, so the pin observes
// what the engine judged with, not a field New set. Kills `stage dropped from
// the default probe`, `default probe not handed to the engine`.
func TestRun_DefaultProbeHonorsPhaseIO(t *testing.T) {
	// A section-complete build report whose only possible violation is the
	// missing failure block (FAIL sentinel without a structured block).
	report := "# Report\n\n## Changes\n- x\n\n" + phasecontract.RenderVerdictSentinel("build", "FAIL") + "\n"
	run := func(stage config.Stage) (core.PhaseResponse, *fakeHooks) {
		t.Helper()
		hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "sonnet", prompt: "x", verdict: core.VerdictPASS, nextPhase: "audit"}
		r := New(Options{Hooks: hooks, Bridge: &goldenBridge{fileContent: report, stdout: panePASS}, Prompts: fakePromptsFS("evolve-builder", "x"),
			PhaseIO: stage, SleepFn: func(time.Duration) {}})
		resp, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: t.TempDir()})
		if err != nil {
			t.Fatalf("PhaseIO=%v: %v", stage, err)
		}
		return resp, hooks
	}
	enforce, hooks := run(config.StageEnforce)
	if hooks.gotArtifact != report {
		t.Fatalf("the contracted report is the verdict source: %q", hooks.gotArtifact)
	}
	if enforce.Verdict != core.VerdictFAIL || enforce.NextPhase != "audit" {
		t.Errorf("at PhaseIO=enforce the engine's default probe must catch a build FAIL-without-block and the guard must downgrade the hook's PASS: %+v", enforce)
	}
	if got := diagMessages(enforce.Diagnostics); len(got) != 1 || !strings.HasPrefix(got[0], "deliverable contract violation ["+deliverable.CodeFailureContextMissing+"]") {
		t.Errorf("the violation the probe found is on the response: %q", got)
	}
	off, hooks := run(config.StageOff)
	if hooks.gotArtifact != report || off.Verdict != core.VerdictPASS || len(off.Diagnostics) != 0 {
		t.Errorf("at PhaseIO=off (default) the default probe must stay dormant (byte-identical PASS, no violation): %+v", off)
	}
}

// TestRun_DefaultProbeResolvesTheMergedCatalog — the reconcile default must
// resolve contracts under the SAME policy as the host gate and the agent
// self-check (merged catalog), not BuiltinResolver-only: a user/minted phase
// (e.g. an advisor-inserted mutation-gate) whose artifact survived a bridge
// timeout was unresolvable and synthesized FAIL. Pinned THROUGH Run (review
// fold F4): a resolved contract makes the on-disk report the verdict source;
// an unresolved one leaves the pane. Kills `BuiltinResolver-only default`.
func TestRun_DefaultProbeResolvesTheMergedCatalog(t *testing.T) {
	root := t.TempDir()
	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(`{"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(root, ".evolve", "phases", "widget-scan")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `{"name":"widget-scan","archetype":"evaluate","agent":"evolve-widget-scan",
		"outputs":{"files":[".evolve/runs/cycle-{cycle}/widget-scan-report.md"]}}`
	if err := os.WriteFile(filepath.Join(userDir, "phase.json"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	report := "# widget-scan\n" + phasecontract.RenderVerdictSentinel("widget-scan", "PASS") + "\n"
	hooks := &fakeHooks{phase: "widget-scan", agent: "evolve-widget-scan", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &goldenBridge{fileContent: report, stdout: panePASS}, Prompts: fakePromptsFS("evolve-widget-scan", "x"), SleepFn: func(time.Duration) {}})
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if _, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 7, ProjectRoot: root, Workspace: ws, Worktree: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if hooks.gotArtifact != report {
		t.Errorf("the default probe must resolve a user phase via the merged catalog (reconcile parity with the gate + self-check) so its report, not the pane, is classified; Classify got %q", hooks.gotArtifact)
	}
}
