package deliverable

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// stubPhase is a PhaseRunner that records its requests and PASSes; onRun, when
// set, plays the agent (writes the artifact).
type stubPhase struct {
	name     string
	requests []core.PhaseRequest
	onRun    func(n int, req core.PhaseRequest)
}

func (s *stubPhase) Name() string { return s.name }
func (s *stubPhase) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	s.requests = append(s.requests, req)
	if s.onRun != nil {
		s.onRun(len(s.requests), req)
	}
	return core.PhaseResponse{Phase: s.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// auditOnly scopes the real contract reviewer to the audit deliverable so the
// stub phases (which write no artifacts) are not what this proof is about.
type auditOnly struct{ inner core.DeliverableReviewer }

func (a auditOnly) Review(ctx context.Context, in core.ReviewInput) core.ReviewResult {
	if in.Phase != string(core.PhaseAudit) {
		return core.ReviewResult{Approve: true}
	}
	return a.inner.Review(ctx, in)
}

// TestLadder_AuditWithoutExplanationSectionIsCorrectedNotFailed is the
// end-to-end proof: a REAL cycle (production storage + ledger, the explanation
// contract active by default) whose auditor first writes a report without
// "## Explanation Documentation" is rejected by the REAL contract reviewer at
// enforce, re-dispatched once with the section named in its directive, and —
// the second report carrying the section — proceeds to ship. Remove the
// registry's ExplanationSections entry or the Roots plumbing and this goes red.
func TestLadder_AuditWithoutExplanationSectionIsCorrectedNotFailed(t *testing.T) {
	root := gitRepoWithOneCommit(t) // the explanation contract seals against a real base SHA
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The builder changes nothing, so its explanation declaration is
	// NOT_APPLICABLE — that satisfies the mandatory build explanation floor and
	// activates the contract for the audit that follows.
	build := &stubPhase{name: string(core.PhaseBuild)}
	build.onRun = func(_ int, req core.PhaseRequest) {
		report := "# Build Report\n\n## Changes\nnone\n\n## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: no material change in this fixture cycle\n"
		if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	audit := &stubPhase{name: string(core.PhaseAudit)}
	audit.onRun = func(n int, req core.PhaseRequest) {
		report := "# Audit Report\n\n## Verdict\n**PASS**\n\n## Issues\nnone\n\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"
		if n > 1 { // the correction round adds the section
			report += "\n## Explanation Documentation\n- Status: VERIFIED\n- Evidence: docs/explain/builds/cycle-1.md:1 matches go/app.go:19\n"
		}
		if err := os.WriteFile(filepath.Join(req.Workspace, "audit-report.md"), []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runners := map[core.Phase]core.PhaseRunner{core.PhaseAudit: audit, core.PhaseBuild: build}
	for _, p := range []core.Phase{core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseShip, core.PhaseRetro} {
		runners[p] = &stubPhase{name: string(p)}
	}
	reviewer := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "breaker.json"), 3)
	o := core.NewOrchestrator(storage.New(evolveDir), ledger.New(evolveDir), runners, core.WithReviewer(auditOnly{reviewer}))
	res, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if len(audit.requests) != 2 {
		t.Fatalf("audit dispatched %d time(s), want 2 (the rejection is a correction, not a terminal FAIL); phases=%v", len(audit.requests), res.PhasesRun)
	}
	if got := audit.requests[1].CorrectionDirective; !strings.Contains(got, `required section "## Explanation Documentation" is missing`) || !strings.Contains(got, "contract v1 is active") {
		t.Errorf("the re-dispatch directive must be the contract gate's reason, got %q", got)
	}
	if ship := runners[core.PhaseShip].(*stubPhase); len(ship.requests) == 0 {
		t.Errorf("the corrected audit must let the cycle reach ship; phases=%v", res.PhasesRun)
	}
}

func gitRepoWithOneCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// The real repo ignores its runtime state and build outputs; without these
	// rules the orchestrator's own files would read as a material Build diff.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".evolve/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", ".gitignore"}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}
