//go:build integration

package ship

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhaseRun_NativeDispatch_Ships(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nnative dispatch\n")
	seedAudit(t, repo, "PASS")
	addRemote(t, repo)

	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 1,
		RunID: "test-run", AuditRound: 1,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-1"),
		Context:     map[string]string{"commit_message": "feat: native dispatch ship"},
		Env:         map[string]string{"EVOLVE_PLUGIN_ROOT": repo},
	})
	if err != nil {
		t.Fatalf("native dispatch errored: %v (diags=%v)", err, resp.Diagnostics)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("want VerdictPASS, got %q (diags=%v)", resp.Verdict, resp.Diagnostics)
	}
	if resp.NextPhase != string(core.PhaseRetro) {
		t.Errorf("want NextPhase=retro, got %q", resp.NextPhase)
	}
}

func TestPhaseRun_NativeDispatch_NoAuditor_FailVerdict(t *testing.T) {
	repo := makeRepo(t) // no seedAudit → audit binding fails

	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-1"),
		Context:     map[string]string{"commit_message": "feat: no auditor"},
		Env:         map[string]string{"EVOLVE_PLUGIN_ROOT": repo},
	})
	if err == nil {
		t.Fatal("want error from native dispatch with no auditor entry")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("want VerdictFAIL, got %q", resp.Verdict)
	}
	if len(resp.Diagnostics) == 0 {
		t.Error("want diagnostics explaining the refusal")
	}
}
