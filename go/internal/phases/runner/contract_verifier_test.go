package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Cycle 1685 (2026-09-15): the engine verified with the plain verifier and
// classified the unrepaired bytes ("no parseable verdict → FAIL"); the gate
// then salvaged, persisted and approved the repaired file, and a red_count=0
// cycle sealed FAIL with no failure class. With the gate's own Reviewer as the
// engine's verifier there is ONE verifier: the salvage happens before
// classification, the classifier sees the repaired sentinel, the result is
// OK (no ship-guard downgrade), and the file on disk is what was judged.
func TestRun_ContractVerifierSalvagesBeforeClassification(t *testing.T) {
	root := writeFallbackProfile(t, "evolve-auditor", "claude-tmux", nil)
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	const fenced = "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n## Ledger Entry\n\n```json\n{\"verdict\": \"PASS\", \"red\": 0}\n```\n"
	bridge := &divergentBridge{fileContent: fenced, stdoutContent: fenced}
	reviewer := deliverable.NewReviewerStage(config.StageEnforce, config.StageEnforce)
	r := New(Options{
		Hooks: hooks, Bridge: bridge, Prompts: fakePromptsFS("evolve-auditor", "x"),
		ContractVerifier: func() ContractVerifier { return reviewer }, PhaseIO: config.StageEnforce,
	})
	if !r.ContractVerifierWired() {
		t.Fatal("the injected contract verifier is the engine's verifier (wiring proof)")
	}
	ws := t.TempDir()
	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: ws})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v, ok := phasecontract.ParseVerdictSentinel(hooks.gotArtifact); !ok || v != "PASS" {
		t.Fatalf("Classify received the REPAIRED bytes (a parseable PASS sentinel), got ok=%v v=%q:\n%s", ok, v, hooks.gotArtifact)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("the salvaged deliverable verifies OK, so no ship-guard downgrade: verdict=%s diags=%+v", resp.Verdict, resp.Diagnostics)
	}
	disk, _ := os.ReadFile(filepath.Join(ws, "audit-report.md"))
	if string(disk) != hooks.gotArtifact || strings.Contains(string(disk), "```json") {
		t.Fatalf("the file on disk is the repaired, classified bytes")
	}
}
