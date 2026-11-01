//go:build integration

package shiptest_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestPhaseRun_NativeDispatch_Ships(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.MustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\nnative dispatch\n")
	shiptest.SeedAudit(t, repo, "PASS")
	shiptest.AddRemote(t, repo)

	p := ship.New(ship.Config{Runner: sysexec.DefaultRunner})
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
