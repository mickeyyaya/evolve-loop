package deliverable

// declared_deliverables_e2e_test.go — ADR-0100, the proof at the public seam.
//
// A REAL cycle (production storage + ledger, the catalog-aware contract
// reviewer at enforce) whose builder writes a contract-valid build-report.md
// and NO handoff-build.json — a secondary the fixture catalog declares the
// agent owes, the way the registry declares triage-decision.json. Before ADR-0100 the cycle proceeded
// to audit. Now: the gate rejects, the ladder re-dispatches with the file
// named in the directive, and either the correction lands (the cycle ships)
// or the ladder exhausts and the cycle ends FAILED_EXPLAINED naming the file.
// Both entrypoints, because the resume loop is a separate implementation.

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
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// buildOnly scopes the real reviewer to the build deliverable: the stub
// phases write no artifacts and are not what this proof is about.
type buildOnly struct{ inner core.DeliverableReviewer }

func (b buildOnly) Review(ctx context.Context, in core.ReviewInput) core.ReviewResult {
	if in.Phase != string(core.PhaseBuild) {
		return core.ReviewResult{Approve: true}
	}
	return b.inner.Review(ctx, in)
}

const contractValidBuildReport = "# Build Report\n\n## Changes\nnone\n\n## Handoff Summary\n- nothing to hand off\n\n## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: no material change in this fixture cycle\n"

// declaredRunners builds a spine whose builder writes the report on every call
// and the handoff only from attempt `handoffFrom` (0 = never).
func declaredRunners(t *testing.T, handoffFrom int) (map[core.Phase]core.PhaseRunner, *stubPhase) {
	t.Helper()
	build := &stubPhase{name: string(core.PhaseBuild)}
	build.onRun = func(n int, req core.PhaseRequest) {
		// A real cycle mints <workspace>/challenge-token.txt and the Build
		// contract requires the report to echo it (proof-of-read); a fixture
		// builder that did not would be rejected for the wrong reason.
		report := contractValidBuildReport
		if tok, err := os.ReadFile(filepath.Join(req.Workspace, "challenge-token.txt")); err == nil {
			report = "<!-- challenge-token: " + strings.TrimSpace(string(tok)) + " -->\n" + report
		}
		if err := os.WriteFile(filepath.Join(req.Workspace, "build-report.md"), []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
		if handoffFrom > 0 && n >= handoffFrom {
			if err := os.WriteFile(filepath.Join(req.Workspace, "handoff-build.json"), []byte(`{"task":"fixture","files":[]}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	runners := map[core.Phase]core.PhaseRunner{core.PhaseBuild: build}
	for _, p := range []core.Phase{core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseAudit, core.PhaseShip, core.PhaseRetro} {
		runners[p] = &stubPhase{name: string(p)}
	}
	return runners, build
}

// fixtureCatalog declares, for build alone, a secondary the agent owes. The
// mechanism is proved against this declaration so the test does not depend on
// what the checked-in registry happens to declare (the registry's own
// classification is pinned by the phasecontract projection tests). The
// built-in Build contract still governs the primary; the overlay adds the
// owed secondary exactly as the registry would.
func fixtureCatalog(t *testing.T) phasespec.Catalog {
	t.Helper()
	cat, warnings := (phasespec.Catalog{}).Merge([]phasespec.PhaseSpec{{
		Name: "build", Role: "build",
		Outputs: phasespec.IO{
			Files:     []string{".evolve/runs/cycle-{cycle}/build-report.md", ".evolve/runs/cycle-{cycle}/handoff-build.json"},
			AgentOwed: []string{"handoff-build.json"},
		},
	}})
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	return cat
}

func declaredOrchestrator(t *testing.T, root string, runners map[core.Phase]core.PhaseRunner) (*core.Orchestrator, core.Storage) {
	t.Helper()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cat := fixtureCatalog(t)
	reviewer := NewReviewerWithCatalog(config.StageEnforce, cat).(*Reviewer)
	reviewer.threshold = 99 // the breaker is not what this proof is about
	reviewer.breakerPath = filepath.Join(t.TempDir(), "breaker.json")
	store := storage.New(evolveDir)
	return core.NewOrchestrator(store, ledger.New(evolveDir), runners, core.WithCatalog(cat), core.WithReviewer(buildOnly{reviewer})), store
}

func correctionNaming(reqs []core.PhaseRequest, file string) int {
	n := 0
	for _, r := range reqs {
		if strings.Contains(r.CorrectionDirective, file) {
			n++
		}
	}
	return n
}

func TestDeclaredDeliverables_MissingHandoff_IsCorrectedThenFails(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	runners, build := declaredRunners(t, 0)
	o, _ := declaredOrchestrator(t, root, runners)

	_, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true})

	if err == nil {
		t.Fatalf("a build that never writes its declared handoff-build.json let the cycle continue — the exact gap ADR-0100 closes; build ran %d time(s)", len(build.requests))
	}
	if len(build.requests) < 2 || correctionNaming(build.requests[1:], "handoff-build.json") == 0 {
		t.Fatalf("the ladder must re-dispatch build with a directive naming handoff-build.json; requests=%d directives=%q", len(build.requests), directives(build.requests))
	}
	if ship := runners[core.PhaseShip].(*stubPhase); len(ship.requests) != 0 {
		t.Errorf("ship ran %d time(s) after an exhausted deliverable correction", len(ship.requests))
	}
	ws := findWorkspace(t, root)
	outcome, detail := cyclehealth.ClassifyOutcome(ws)
	if outcome != cyclehealth.OutcomeFailedExplained || !strings.Contains(detail, "handoff-build.json") {
		t.Fatalf("outcome=%q detail=%q; want FAILED_EXPLAINED naming handoff-build.json", outcome, detail)
	}
}

func TestDeclaredDeliverables_CorrectedHandoff_IsAccepted(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	runners, build := declaredRunners(t, 2) // the correction round writes it
	o, _ := declaredOrchestrator(t, root, runners)

	if _, err := o.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("a corrected deliverable must be accepted: %v", err)
	}
	if len(build.requests) != 2 {
		t.Fatalf("build ran %d time(s), want exactly 2 (one correction)", len(build.requests))
	}
	if correctionNaming(build.requests[1:], "handoff-build.json") != 1 {
		t.Fatalf("the single correction must name the file: %q", directives(build.requests))
	}
	if ship := runners[core.PhaseShip].(*stubPhase); len(ship.requests) != 1 {
		t.Errorf("the corrected cycle must reach ship exactly once, got %d", len(ship.requests))
	}
}

// TestDeclaredDeliverables_Resume_MissingHandoff_IsCorrectedThenFails is the
// resume twin: RunCycleFromPhase is a separate loop, and this session's
// resume-parity fixes (#568, #571) are why the twin is not optional.
func TestDeclaredDeliverables_Resume_MissingHandoff_IsCorrectedThenFails(t *testing.T) {
	root := gitRepoWithOneCommit(t)
	runners, build := declaredRunners(t, 0)
	o, store := declaredOrchestrator(t, root, runners)
	ctx := context.Background()
	ws := core.RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	sha, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(sha))
	if err := store.WriteCycleState(ctx, core.CycleState{CycleID: 7, RunID: "resume-7", GoalHash: "g",
		WorkspacePath: ws, ActiveWorktree: root, WorktreeBaseSHA: base,
		ExplanationDocumentationVersion: 1, CompletedPhases: []string{"scout", "triage", "tdd"}}); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.Activate(explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: root, Workspace: ws, BaseSHA: base, Cycle: 7, RunID: "resume-7", ContractVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}

	_, runErr := o.RunCycleFromPhase(ctx, core.CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true},
		&core.ResumePoint{CycleID: 7, Phase: string(core.PhaseBuild), WorktreePath: root})

	if runErr == nil {
		t.Fatalf("resume: a build that never writes handoff-build.json let the cycle continue; build ran %d time(s)", len(build.requests))
	}
	if len(build.requests) < 2 || correctionNaming(build.requests[1:], "handoff-build.json") == 0 {
		t.Fatalf("resume: the ladder must re-dispatch build naming handoff-build.json; requests=%d directives=%q", len(build.requests), directives(build.requests))
	}
	outcome, detail := cyclehealth.ClassifyOutcome(ws)
	if outcome != cyclehealth.OutcomeFailedExplained || !strings.Contains(detail, "handoff-build.json") {
		t.Fatalf("resume: outcome=%q detail=%q; want FAILED_EXPLAINED naming handoff-build.json", outcome, detail)
	}
}

func directives(reqs []core.PhaseRequest) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, r.CorrectionDirective)
	}
	return out
}

func findWorkspace(t *testing.T, root string) string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "runs", "cycle-*"))
	if len(matches) != 1 {
		t.Fatalf("want exactly one cycle workspace, got %v", matches)
	}
	return matches[0]
}
