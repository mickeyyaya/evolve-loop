package runner

// runner_verify_roots_cycle_test.go — ADR-0100 slice 2: the runner's own
// verify (the classification check and the teardown reconcile) hands the
// verifier the SAME cycle the host gate and the agent self-check use, so a
// declared effect (judged under processing/cycle-N/) is decided by all three
// with one belief. A runner that omitted it would make the verifier fail OPEN
// on every effect-declaring phase — silently keeping the pane as the verdict
// source.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestRun_VerifyRootsCarryTheCycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		bridgeErr error
	}{
		{"completed phase (classification verify)", nil},
		{"bridge timeout (teardown reconcile verify)", artifactTimeoutErr()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen []phasecontract.Roots
			capture := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
				seen = append(seen, roots)
				return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
			}
			hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
			fb := &fakeBridge{err: tc.bridgeErr, writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
			r := New(Options{Hooks: hooks, Bridge: fb, Prompts: fakePromptsFS("evolve-auditor", "x"), VerifyFn: capture})
			root := t.TempDir()

			if _, err := r.Run(context.Background(), core.PhaseRequest{Cycle: 42, ProjectRoot: root, Workspace: t.TempDir()}); err != nil {
				t.Fatal(err)
			}

			if len(seen) == 0 {
				t.Fatal("the runner never verified the deliverable")
			}
			for _, roots := range seen {
				if roots.Cycle != 42 {
					t.Errorf("runner verify roots carry Cycle=%d, want 42 — a declared effect could not be located and the verifier would fail open", roots.Cycle)
				}
				if roots.EvolveDir != filepath.Join(root, ".evolve") {
					t.Errorf("runner verify roots carry EvolveDir=%q, want %q", roots.EvolveDir, filepath.Join(root, ".evolve"))
				}
			}
		})
	}
}
