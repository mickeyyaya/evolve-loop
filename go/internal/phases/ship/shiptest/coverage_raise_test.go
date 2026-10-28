//go:build integration

package shiptest_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestPhaseRun_DefaultCommitMessage_WhenContextMissing(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.MustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\ndefault-msg path\n")
	shiptest.SeedAudit(t, repo, "PASS", map[string]string{"cycle": "42"})
	shiptest.AddRemote(t, repo)

	p := ship.New(ship.Config{Runner: sysexec.DefaultRunner})
	resp, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 42,
		RunID: "test-run", AuditRound: 1,
		ProjectRoot: repo,
		Workspace:   filepath.Join(repo, ".evolve", "runs", "cycle-42"),
		Env:         map[string]string{"EVOLVE_PLUGIN_ROOT": repo},
	})
	if err != nil {
		t.Fatalf("Run with default message errored: %v (diags=%v)", err, resp.Diagnostics)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("want VerdictPASS, got %q (diags=%v)", resp.Verdict, resp.Diagnostics)
	}
	subject := strings.TrimSpace(shiptest.RunGitOut(t, repo, "log", "-1", "--format=%s"))
	if !strings.Contains(subject, "evolve-cycle 42") {
		t.Errorf("commit subject = %q, want synthesized 'evolve-cycle 42'", subject)
	}
}
