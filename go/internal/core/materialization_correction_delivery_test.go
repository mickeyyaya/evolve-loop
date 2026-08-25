package core_test

// materialization_correction_delivery_test.go — cycle-1554 RED contract for
// `scout-eval-materialization-correction-delivery` (inbox
// pipeline-defect-pipeline-blocker, P0).
//
// evalgate_escalation_test.go (package core) already proves the CLI-escalation
// ladder using a hand-authored evalGateProbe reviewer that FAKES the gate's
// Approve/Reason/Remediation shape. That test never runs the REAL
// evalgate.materializationGate — it never stats a real
// .evolve/evals/<slug>.md file, never reads a real scout-report.md, and never
// verifies that a file the agent creates in response to the correction
// directive actually clears the real gate on re-review. Nothing in either the
// core or evalgate suite drives the production seam end-to-end: real
// scout-report.md (missing eval) -> real materializationGate rejection with
// its real remediation text -> correction re-dispatch -> agent creates the
// eval at the EXACT workspace path the remediation named -> real
// materializationGate re-review APPROVES. This file closes that gap (cycles
// 1540/1545: the remediation text was byte-perfect and the correction still
// burned its full budget without landing — "produced a remediation string
// without a proven consumer" per the scout hypothesis).
//
// External test package (core_test): wiring the REAL evalgate.NewReviewer
// (which itself imports core) alongside core would be an import cycle from an
// internal test file — see internal/core/evalgate_escalation_test.go's header
// for why that file stays a fake instead. test/fixtures supplies the
// canonical FakeStorage/FakeLedger/BuildRunners so this file needs no
// unexported core test helpers.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/evalgate"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// materializationScoutRunner plays the scout phase across a correction
// round-trip: call 1 selects a slug but writes no eval (the live defect
// class); call 2+ writes the eval at the workspace path found in the
// CorrectionDirective it was re-dispatched with — the same way a complying
// agent would after reading the gate's remediation.
type materializationScoutRunner struct {
	slug       string
	requests   []core.PhaseRequest
	createEval bool // when false, NEVER complies — the live 0-for-N shape
}

func (r *materializationScoutRunner) Name() string { return string(core.PhaseScout) }

func (r *materializationScoutRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.requests = append(r.requests, req)
	report := "## Selected Tasks\n\n### Task 1\n- **Slug:** " + r.slug + "\n\n"
	if err := os.WriteFile(filepath.Join(req.Workspace, "scout-report.md"), []byte(report), 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	// A complying re-dispatch creates the eval at exactly the path named in
	// the correction directive — never at any other root (the sandbox
	// projectRoot is deny-write; only the workspace path is both writable and
	// gate-sufficient, per materialization.go's remediation() contract).
	if r.createEval && len(r.requests) > 1 {
		evalPath := filepath.Join(req.Workspace, ".evolve", "evals", r.slug+".md")
		if err := os.MkdirAll(filepath.Dir(evalPath), 0o755); err != nil {
			return core.PhaseResponse{}, err
		}
		body := "# Eval " + r.slug + "\n\n```bash\ngo test ./internal/widget/...\n```\n"
		if err := os.WriteFile(evalPath, []byte(body), 0o644); err != nil {
			return core.PhaseResponse{}, err
		}
	}
	return core.PhaseResponse{Phase: string(core.PhaseScout), Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// runMaterializationCycle wires the REAL evalgate reviewer (production
// composition, not a fake) at enforce, with the given scout runner standing
// in for every other phase runner unchanged.
func runMaterializationCycle(t *testing.T, scout *materializationScoutRunner) error {
	t.Helper()
	root := t.TempDir()
	st := &fixtures.FakeStorage{}
	runners := fixtures.BuildRunners(nil)
	runners[core.PhaseScout] = scout
	reviewer := evalgate.NewReviewer(config.StageEnforce)
	o := core.NewOrchestrator(st, &fixtures.FakeLedger{}, runners, core.WithReviewer(reviewer))
	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	return err
}

// TestMaterializationCorrectionDelivery_RemediationReachesReDispatchAndClearsTheRealGate
// is the headline: the real materializationGate rejects a missing eval,
// names the exact workspace path in the correction directive, and a
// complying re-dispatch that writes the eval there clears the SAME real gate
// on re-review — the production correction round-trip, not a mock of it.
func TestMaterializationCorrectionDelivery_RemediationReachesReDispatchAndClearsTheRealGate(t *testing.T) {
	scout := &materializationScoutRunner{slug: "widget-thing", createEval: true}
	if err := runMaterializationCycle(t, scout); err != nil {
		t.Fatalf("complying correction must clear the real materialization gate: %v", err)
	}

	if len(scout.requests) < 2 {
		t.Fatalf("want at least 2 scout dispatches (initial rejection + complying correction); got %d", len(scout.requests))
	}
	directive := scout.requests[1].CorrectionDirective
	if directive == "" {
		t.Fatalf("second dispatch must carry a non-empty correction directive")
	}
	wantPath := filepath.Join(scout.requests[1].Workspace, ".evolve", "evals", "widget-thing.md")
	if !strings.Contains(directive, wantPath) {
		t.Fatalf("correction directive must name the EXACT workspace eval path %q; got %q", wantPath, directive)
	}
	// The gate's own "no unrelated edits" clause must not survive into a
	// remediation-carrying directive — a remediation exists precisely because
	// the fix is to CREATE a file, and the default clause forbids that.
	if strings.Contains(directive, "Do not change unrelated files.") {
		t.Errorf("a remediation-carrying directive must not also forbid the required file creation: %q", directive)
	}
	// The eval the agent wrote (in response to the directive) must be present
	// at the exact path the gate names — proving the round trip actually
	// closed, not just that a directive was composed.
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("complying correction must have created the eval at %q: %v", wantPath, err)
	}
}

// TestMaterializationCorrectionDelivery_NonComplyingReDispatchStaysRejected —
// the anti-no-op axis: a correction round that does NOT create the eval must
// keep failing the real gate every round (the live 0-for-N shape prior to the
// fix), never silently pass just because a correction round happened.
func TestMaterializationCorrectionDelivery_NonComplyingReDispatchStaysRejected(t *testing.T) {
	scout := &materializationScoutRunner{slug: "widget-thing", createEval: false}
	if err := runMaterializationCycle(t, scout); err == nil {
		t.Fatal("non-complying correction must remain rejected by the real materialization gate")
	}

	if len(scout.requests) < 2 {
		t.Fatalf("want at least 2 scout dispatches even on the non-complying path (correction must still be attempted); got %d", len(scout.requests))
	}
	evalPath := filepath.Join(scout.requests[0].Workspace, ".evolve", "evals", "widget-thing.md")
	if _, err := os.Stat(evalPath); err == nil {
		t.Fatalf("non-complying scout must never have produced the eval file — the test fixture is broken if it exists")
	}
	// Every dispatch's directive must keep naming the same missing path — the
	// signal is not a one-shot warning that silently drops on repeat.
	for i, req := range scout.requests[1:] {
		if !strings.Contains(req.CorrectionDirective, "widget-thing.md") {
			t.Errorf("dispatch %d correction directive lost the missing slug's path: %q", i+1, req.CorrectionDirective)
		}
	}
}
