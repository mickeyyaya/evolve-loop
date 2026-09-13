package deliverable

// declared_effects_e2e_test.go — ADR-0100 slice 2, the proof at the public
// seam.
//
// A REAL cycle (production storage + ledger, the catalog-aware contract
// reviewer at enforce) whose triage writes a contract-valid triage-report.md
// and triage-decision.json committing to inbox item "x" — and never claims
// it: the item stays pending at the plane's inbox root, exactly the 1631
// shape (the agent "claimed" a worktree copy) and the 1623 shape (the claim
// was denied and the spine ran anyway). Before this slice the cycle proceeded
// to tdd. Now: the gate rejects, the ladder re-dispatches with the item and
// the command named in the directive, and either the claim lands (the cycle
// ships) or the ladder exhausts and the cycle ends FAILED_EXPLAINED naming
// the effect. An empty commitment owes no claim and still ends as triage
// no-work.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// triageOnly scopes the real reviewer to the triage deliverable: the stub
// phases write no artifacts and are not what this proof is about.
type triageOnly struct{ inner core.DeliverableReviewer }

func (r triageOnly) Review(ctx context.Context, in core.ReviewInput) core.ReviewResult {
	if in.Phase != string(core.PhaseTriage) {
		return core.ReviewResult{Approve: true}
	}
	return r.inner.Review(ctx, in)
}

const pendingItemFile = "2026-09-12T00-00-00Z-x.json"

// seedPendingItem drops inbox item "x" at the plane's inbox root.
func seedPendingItem(t *testing.T, root string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, pendingItemFile), []byte(`{"id":"x","title":"fixture","weight":0.9}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// effectRunners builds a spine whose triage commits to "x" on every call and
// claims it (through the production inboxmover.Claim) only from attempt
// claimFrom (0 = never). commit=false writes an
// explicit empty commitment instead.
func effectRunners(t *testing.T, claimFrom int, commit bool) (map[core.Phase]core.PhaseRunner, *stubPhase) {
	t.Helper()
	triage := &stubPhase{name: string(core.PhaseTriage)}
	triage.onRun = func(n int, req core.PhaseRequest) {
		report := "# Triage Decision — Cycle " + strconv.Itoa(req.Cycle) + "\n\n## top_n\n"
		decision := `{"cycle":` + strconv.Itoa(req.Cycle) + `,"top_n":[],"deferred":[]}`
		if commit {
			report += "- x: fix the thing — priority=H, files=a.go, source=inbox\n"
			decision = `{"cycle":` + strconv.Itoa(req.Cycle) + `,"top_n":[{"id":"x"}],"deferred":[]}`
		}
		if tok, err := os.ReadFile(filepath.Join(req.Workspace, "challenge-token.txt")); err == nil {
			report = "<!-- challenge-token: " + strings.TrimSpace(string(tok)) + " -->\n" + report
		}
		for name, body := range map[string]string{"triage-report.md": report, "triage-decision.json": decision} {
			if err := os.WriteFile(filepath.Join(req.Workspace, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if claimFrom > 0 && n >= claimFrom {
			// The persona's `evolve inbox-mover claim` — the production writer, so
			// the proof holds only if the gate reads where the claim really lands.
			if _, err := inboxmover.Claim(inboxmover.Options{ProjectRoot: req.ProjectRoot, Stderr: io.Discard}, "x", strconv.Itoa(req.Cycle)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The build floor (a separate deterministic gate) needs a contract-valid
	// build-report.md to let the corrected cycle reach ship; reuse PR-1's.
	build := &stubPhase{name: string(core.PhaseBuild)}
	build.onRun = func(_ int, req core.PhaseRequest) {
		report := contractValidBuildReport
		if tok, err := os.ReadFile(filepath.Join(req.Workspace, "challenge-token.txt")); err == nil {
			report = "<!-- challenge-token: " + strings.TrimSpace(string(tok)) + " -->\n" + report
		}
		if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runners := map[core.Phase]core.PhaseRunner{core.PhaseTriage: triage, core.PhaseBuild: build}
	for _, p := range []core.Phase{core.PhaseScout, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
		runners[p] = &stubPhase{name: string(p)}
	}
	return runners, triage
}

// effectCatalog is the registry's triage declaration in miniature — the owed
// decision plus the claim effect — so the proof does not depend on what the
// checked-in registry happens to declare (that is pinned by
// TestPhaseRegistry_EveryDeclaredEffectHasACheck).
func effectCatalog(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{{
		Name: "triage", Role: "triage",
		Outputs: phasespec.IO{
			Files:     []string{".evolve/runs/cycle-{cycle}/triage-report.md", ".evolve/runs/cycle-{cycle}/triage-decision.json"},
			AgentOwed: []string{"triage-decision.json"},
		},
		Effects: []string{"inbox-claim"},
	}})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	return cat
}

func effectOrchestrator(t *testing.T, root string, runners map[core.Phase]core.PhaseRunner) *core.Orchestrator {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cat := effectCatalog(t)
	reviewer := NewReviewerWithCatalog(config.StageEnforce, cat).(*Reviewer)
	reviewer.threshold = 99 // the breaker is not what this proof is about
	reviewer.breakerPath = filepath.Join(t.TempDir(), "breaker.json")
	return core.NewOrchestrator(storage.New(evolveDir), ledger.New(evolveDir), runners, core.WithCatalog(cat), core.WithReviewer(triageOnly{reviewer}))
}

func TestDeclaredEffects_CommittedItemNotClaimed_IsCorrectedThenFails(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	seedPendingItem(t, root)
	runners, triage := effectRunners(t, 0, true)
	o := effectOrchestrator(t, root, runners)

	_, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})

	if err == nil {
		t.Fatalf("a triage that committed to inbox item x without claiming it let the cycle continue — the 1623/1631 shape; triage ran %d time(s)", len(triage.requests))
	}
	if len(triage.requests) < 2 || correctionNaming(triage.requests[1:], "inbox-claim") == 0 || correctionNaming(triage.requests[1:], `"x"`) == 0 {
		t.Fatalf("the ladder must re-dispatch triage with a directive naming the effect and the item; requests=%d directives=%q", len(triage.requests), directives(triage.requests))
	}
	if tdd := runners[core.PhaseTDD].(*stubPhase); len(tdd.requests) != 0 {
		t.Errorf("tdd ran %d time(s) on an unclaimed commitment", len(tdd.requests))
	}
	outcome, detail := cyclehealth.ClassifyOutcome(findWorkspace(t, root))
	if outcome != cyclehealth.OutcomeFailedExplained || !strings.Contains(detail, "inbox-claim") {
		t.Fatalf("outcome=%q detail=%q; want FAILED_EXPLAINED naming inbox-claim", outcome, detail)
	}
}

func TestDeclaredEffects_ClaimedCommitment_IsAccepted(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	seedPendingItem(t, root)
	runners, triage := effectRunners(t, 2, true) // the correction round claims it
	o := effectOrchestrator(t, root, runners)

	if _, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("a claimed commitment must be accepted: %v", err)
	}
	if len(triage.requests) != 2 || correctionNaming(triage.requests[1:], "inbox-claim") != 1 {
		t.Fatalf("triage ran %d time(s), want exactly 2 with the one correction naming inbox-claim: %q", len(triage.requests), directives(triage.requests))
	}
	if ship := runners[core.PhaseShip].(*stubPhase); len(ship.requests) != 1 {
		t.Errorf("the corrected cycle must reach ship exactly once, got %d", len(ship.requests))
	}
}

func TestDeclaredEffects_EmptyCommitment_OwesNoClaim(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	seedPendingItem(t, root) // pending, and legitimately left alone
	runners, triage := effectRunners(t, 0, false)
	o := effectOrchestrator(t, root, runners)

	result, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})

	if err != nil || len(triage.requests) != 1 || correctionNaming(triage.requests, "inbox-claim") != 0 {
		t.Fatalf("an explicit empty commitment owes no claim: err=%v triage runs=%d directives=%q", err, len(triage.requests), directives(triage.requests))
	}
	if !core.IsTriageNoWorkResult(result) {
		t.Fatalf("an empty commitment still ends as triage no-work, got verdict=%q termination=%q phases=%v", result.FinalVerdict, result.TerminationReason, result.PhasesRun)
	}
}
