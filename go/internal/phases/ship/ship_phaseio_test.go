package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
)

// ADR-0050 §3.10 Slice 4: ship reads commit_message from the typed envelope at
// enforce (req.Input.Active()) and the legacy Context["commit_message"] below it.
// The empty→defaultCommitMessage(req) fallback (cycle-150 lesson) is preserved on
// both paths. We assert the resolved message reached the commit via HEAD's subject,
// matching the existing TestPhaseRun_DefaultCommitMessage_WhenContextMissing seam.

func shipOnce(t *testing.T, req core.PhaseRequest) string {
	t.Helper()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture\nphaseio slice4\n")
	seedAudit(t, repo, "PASS")
	addRemote(t, repo)
	req.ProjectRoot = repo
	req.Workspace = filepath.Join(repo, ".evolve", "runs", "cycle-1")
	if req.Env == nil {
		req.Env = map[string]string{}
	}
	req.Env["EVOLVE_PLUGIN_ROOT"] = repo
	p := New(Config{Runner: execRunner})
	resp, err := p.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("ship Run errored: %v (diags=%v)", err, resp.Diagnostics)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Fatalf("want VerdictPASS, got %q (diags=%v)", resp.Verdict, resp.Diagnostics)
	}
	return strings.TrimSpace(runGitOut(t, repo, "log", "-1", "--format=%s"))
}

func TestShip_CommitMessage_TypedVsMap(t *testing.T) {
	typedEnvelope := func(msg string) phaseio.PhaseInput {
		return phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "ship",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{CommitMessage: msg}),
		})
	}

	// Legacy map path (below enforce): Context["commit_message"] wins.
	t.Run("map_path", func(t *testing.T) {
		subject := shipOnce(t, core.PhaseRequest{
			Cycle:   7,
			Context: map[string]string{"commit_message": "feat: from the map"},
		})
		if !strings.Contains(subject, "feat: from the map") {
			t.Errorf("map path: subject = %q, want the Context message", subject)
		}
	})

	// Enforce path: the typed envelope is consulted even with NO Context. RED before
	// the fix — map read returns "" → defaultCommitMessage → subject "evolve-cycle 7".
	t.Run("typed_path_enforce", func(t *testing.T) {
		subject := shipOnce(t, core.PhaseRequest{
			Cycle: 7,
			Input: typedEnvelope("feat: from the envelope"),
		})
		if !strings.Contains(subject, "feat: from the envelope") {
			t.Errorf("typed path: subject = %q, want the typed envelope message", subject)
		}
	})

	// An active envelope with an EMPTY CommitMessage still falls back to the
	// synthesized cycle message — byte-identical to the legacy empty-map default.
	t.Run("typed_empty_falls_to_default", func(t *testing.T) {
		subject := shipOnce(t, core.PhaseRequest{
			Cycle: 42,
			Input: typedEnvelope(""),
		})
		if !strings.Contains(subject, "evolve-cycle 42") {
			t.Errorf("empty typed message: subject = %q, want synthesized 'evolve-cycle 42'", subject)
		}
	})
}
