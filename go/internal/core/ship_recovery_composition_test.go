// ship_recovery_composition_test.go — RED contract for wiring the RUNG 0
// composition-verdict writer into the live fleet-rebase recovery path
// (cycle 801, inbox weight 0.98, campaign merge-efficiency-2026-07).
//
// Ship's trivial-rebase carry-forward reader (internal/phases/ship/
// composition.go) and the ledger's kernel-recomputable writer
// (internal/adapters/ledger/composition.go) are both fully built and unit-
// tested (cycle-786), but nothing in production code ever calls
// ledger.WriteCompositionVerdict — the one call site that could produce a
// composition-verdict entry for a CodeGitFleetRebaseNeeded recovery
// (recoverFromShipError, ship_recovery.go:45) always falls through to a
// full re-audit (router.Recover routes GIT_FLEET_REBASE_NEEDED → "audit"
// unconditionally, router/recovery.go:110). Note internal/core cannot
// import internal/adapters/ledger directly (ledger already imports core —
// an import cycle), so the fast path must be wired the same way core
// already wires catalogRefresh/modelCatalogLookup/directivesProvider:
// exported Option-injected closures the composition root (cmd/evolve)
// binds to the real ledger adapter; core itself stays adapter-agnostic.
//
// This file pins the OBSERVABLE, black-box contract (package core_test,
// driven only through the public core.NewOrchestrator/RunCycle API — it
// does not prescribe recoverFromShipError's internal signature, only the
// new exported seams and the routing behavior Builder must make true):
//
//  1. Two new exported types:
//     - core.CompositionAuditSnapshot{LaneAuditRef, AuditedBase string;
//     Diff []byte; PatchID string} — what the bound audit reviewed
//     for this lane BEFORE a peer moved main.
//     - core.CompositionVerdictInput — mirrors
//     ledger.CompositionVerdictInput field-for-field so the composition
//     root's injected writer closure can translate 1:1 into a real
//     ledger.WriteCompositionVerdict call.
//  2. Three new exported Options, each nil by default (⇒ the composition
//     fast path never fires; recovery behaves exactly as it does today):
//     - core.WithCompositionSnapshot(func(ctx, worktree, runID string)
//     (core.CompositionAuditSnapshot, error)) — captures the lane's
//     pre-rebase audited state.
//     - core.WithCompositionGateRunner(func(ctx, worktree string)
//     map[string]string) — runs the full native composed-tree gate set
//     (ciparity.RequiredComposedGates) on the rebased tree.
//     - core.WithCompositionVerdictWriter(func(ledgerPath string,
//     in core.CompositionVerdictInput) error) — persists the entry.
//  3. On a CLEAN fleet rebase for CodeGitFleetRebaseNeeded (the existing
//     rebaseCycleBranchOntoMain ok==true branch), when all three seams are
//     wired: the composed diff's recomputed patch-id must match the
//     snapshot's PatchID (drift → fall back, unchanged — same semantic-
//     drift guard ship's own tryTrivialRebaseCarryForward already
//     enforces on read) AND every ciparity.RequiredComposedGates entry
//     must be "pass" (ciparity.MissingComposedGates nil) before the writer
//     is ever called. Only when the writer succeeds does recovery route
//     straight back to ship WITHOUT re-running audit. Any rejection
//     (missing seam, patch-id drift, red gate, writer error) falls through
//     to the pre-existing full re-audit route unchanged — the fast path
//     can only narrow, never widen, what ships.
//
// RED today via this file's compile dependency on the not-yet-defined
// exported symbols above (same convention as ship_recovery_width_test.go's
// white-box RED).
package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

// errWriteBoom is the forced compositionVerdictWriter failure used by
// TestRecoverFromShipError_CleanRebase_WriterFailureFallsBackToFullAudit.
var errWriteBoom = errors.New("ledger write boom")

// writeFileT writes content to path, creating parent dirs as needed.
func writeFileT(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// fixedWorktree is a core.WorktreeProvisioner fake that always hands back
// the same pre-built repo dir, so recoverFromShipError's real
// `git rebase main` runs against a repo this suite fully controls.
type fixedWorktree struct{ dir string }

func (f fixedWorktree) Create(string, int) (string, error) { return f.dir, nil }
func (f fixedWorktree) Cleanup(string, string) error       { return nil }

// countingRunner is a PhaseRunner that always PASSes and counts its calls —
// used here as the audit runner so tests can assert whether a composition
// carry-forward actually SKIPPED re-audit (calls stay 0) or fell back to the
// pre-existing full-audit route (calls become 1).
type countingRunner struct {
	name  string
	calls int
}

type explanationWritingRunner struct {
	calls int
}

func (r *explanationWritingRunner) Name() string { return string(core.PhaseBuild) }
func (r *explanationWritingRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	diff := runGitOutput(req.Worktree, "diff", "--name-only", req.WorktreeBaseSHA, "--")
	report := "## Explanation Documentation\n- Status: NOT_APPLICABLE\n- Reason: the base-bound Build diff contains no material implementation changes\n"
	if strings.Contains("\n"+diff+"\n", "\nlane.txt\n") {
		doc, err := explanationdocs.DocumentPath(req.Cycle, req.RunID)
		if err != nil {
			return core.PhaseResponse{}, err
		}
		body := "# Build Explanation\n\n## Build Binding\n- Cycle: " + fmt.Sprint(req.Cycle) + "\n- Base SHA: " + req.WorktreeBaseSHA + "\n\n" +
			"## Summary\nThe lane change is replayed on the latest fleet base.\n\n" +
			"## Rationale\nRebuilding the explanation after rebase keeps its evidence bound to the exact tree that Audit and Ship verify.\n\n" +
			"## Changed Areas\n- `lane.txt` — carries the lane-specific behavior replayed by this cycle.\n\n" +
			"## Design Decisions\nThe lane remains isolated in its existing file and introduces no new abstraction.\n\n" +
			"## Verification\nThe recovery integration suite runs Build, Audit, and Ship against the rebased tree.\n\n" +
			"## Compatibility\nNo public interface changes.\n\n## Limitations\nNo migration is needed.\n"
		if err := writeFileT(filepath.Join(req.Worktree, filepath.FromSlash(doc)), body); err != nil {
			return core.PhaseResponse{}, err
		}
		report = "## Explanation Documentation\n- Status: REQUIRED\n- Document: " + doc + "\n"
	}
	if err := writeFileT(filepath.Join(req.Workspace, "build-report.md"), report); err != nil {
		return core.PhaseResponse{}, err
	}
	return core.PhaseResponse{Phase: string(core.PhaseBuild), Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func runGitOutput(dir string, args ...string) string {
	out, _ := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return strings.TrimSpace(string(out))
}

func (r *countingRunner) Name() string { return r.name }
func (r *countingRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

// greenComposedGateResults returns an all-PASS result for every gate
// ciparity.RequiredComposedGates requires.
func greenComposedGateResults() map[string]string {
	out := map[string]string{}
	for _, g := range ciparity.RequiredComposedGates {
		out[g] = "pass"
	}
	return out
}

// runGitT runs git in dir, failing the test on a non-zero exit.
func runGitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// initCleanRebaseRepoT builds a temp git repo where branch `cycle` and
// `main` diverge on DIFFERENT files, so rebasing `cycle` onto `main`
// replays cleanly (no conflict) — the precondition for the composition
// carry-forward fast path. Leaves `cycle` checked out at the pre-rebase tip
// and returns the dir plus the pre-rebase diff of cycle's own commit
// against the OLD main (what an audit bound before a peer moved main).
func initCleanRebaseRepoT(t *testing.T) (dir string, preRebaseDiff []byte) {
	t.Helper()
	dir = t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		if err := writeFileT(filepath.Join(dir, rel), content); err != nil {
			t.Fatal(err)
		}
	}
	runGitT(t, dir, "init")
	runGitT(t, dir, "checkout", "-b", "main")
	runGitT(t, dir, "config", "user.email", "t@example.com")
	runGitT(t, dir, "config", "user.name", "test")
	runGitT(t, dir, "config", "commit.gpgsign", "false")
	write("base.txt", "base\n")
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "base")
	runGitT(t, dir, "checkout", "-b", "cycle")
	write("lane.txt", "lane change\n")
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "lane change")
	preRebaseDiff = []byte(runGitT(t, dir, "diff", "main...HEAD"))
	runGitT(t, dir, "checkout", "main")
	write("peer.txt", "peer change\n") // disjoint file → clean replay
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "peer change")
	runGitT(t, dir, "checkout", "cycle")
	return dir, preRebaseDiff
}

// compositionCycleFixture bundles the ship/audit stubs and Orchestrator
// wired for one composition-carry-forward scenario.
type compositionCycleFixture struct {
	ship    *shipErrorStub
	audit   *countingRunner
	build   *explanationWritingRunner
	storage *recStorage
	o       *core.Orchestrator
}

// newCompositionFixture wires an Orchestrator whose ship runner fails once
// with CodeGitFleetRebaseNeeded (then PASSes) against repoDir, with the
// three composition seams set from snap/gates/write.
func newCompositionFixture(
	repoDir string,
	snap func(context.Context, string, string) (core.CompositionAuditSnapshot, error),
	gates func(context.Context, string) map[string]string,
	write func(string, core.CompositionVerdictInput) error,
) compositionCycleFixture {
	ship := &shipErrorStub{
		name:      "ship",
		failFirst: 1,
		errOnFail: core.NewShipError(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip, "peer landed during audit->ship gap"),
	}
	audit := &countingRunner{name: "audit"}
	build := &explanationWritingRunner{}
	runners := newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseShip:  ship,
		core.PhaseAudit: audit,
		core.PhaseBuild: build,
	})
	storage := &recStorage{}
	o := core.NewOrchestrator(storage, &fakeLedger{}, runners,
		core.WithWorktreeProvisioner(fixedWorktree{dir: repoDir}),
		core.WithCompositionSnapshot(snap),
		core.WithCompositionGateRunner(gates),
		core.WithCompositionVerdictWriter(write),
	)
	return compositionCycleFixture{ship: ship, audit: audit, build: build, storage: storage, o: o}
}

func runCompositionCycle(t *testing.T, o *core.Orchestrator) error {
	t.Helper()
	_, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "merge-rung0-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	})
	return err
}

// TestRecoverFromShipError_CleanRebase_WritesCompositionVerdictAndSkipsReaudit
// is AC1 (RED): a clean fleet rebase whose recomputed composed patch-id
// matches the audited snapshot AND whose composed-tree gates are all green
// must write a composition-verdict entry and route straight back to ship,
// skipping the full re-audit the router would otherwise force.
func TestRecoverFromShipError_CleanRebase_RebuildsBaseBoundExplanationBeforeReaudit(t *testing.T) {
	dir, preDiff := initCleanRebaseRepoT(t)
	patchID, err := ledger.PatchID(preDiff)
	if err != nil {
		t.Fatalf("compute fixture patch-id: %v", err)
	}
	var wrote []core.CompositionVerdictInput
	fx := newCompositionFixture(dir,
		func(context.Context, string, string) (core.CompositionAuditSnapshot, error) {
			return core.CompositionAuditSnapshot{
				LaneAuditRef: "audit-artifact-sha",
				AuditedBase:  "old-main-sha",
				Diff:         preDiff,
				PatchID:      patchID,
			}, nil
		},
		func(context.Context, string) map[string]string { return greenComposedGateResults() },
		func(ledgerPath string, in core.CompositionVerdictInput) error {
			wrote = append(wrote, in)
			return nil
		},
	)

	if err := runCompositionCycle(t, fx.o); err != nil {
		t.Fatalf("clean rebase + matching patch-id + green gates must NOT abort the cycle, got: %v", err)
	}
	if fx.ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail once → composition carry-forward → reship)", fx.ship.calls)
	}
	if fx.build.calls != 2 || fx.audit.calls != 2 {
		t.Fatalf("calls build=%d audit=%d, want 2/2 — a base-bound explanation must be regenerated and re-audited after rebase", fx.build.calls, fx.audit.calls)
	}
	if len(wrote) != 0 {
		t.Fatalf("composition carry-forward must not bypass base-bound explanation regeneration, writer calls=%d", len(wrote))
	}
}

// TestRecoverFromShipError_CleanRebase_PatchIdDriftFallsBackToFullAudit is
// AC2 (RED): when the pre-rebase audited snapshot's patch-id does NOT match
// what the composed (post-rebase) tree recomputes — the semantic-drift
// guard ship's own tryTrivialRebaseCarryForward already enforces on read —
// the writer must NOT be called and recovery must fall through to the
// pre-existing full re-audit route, unchanged.
func TestRecoverFromShipError_CleanRebase_PatchIdDriftFallsBackToFullAudit(t *testing.T) {
	dir, _ := initCleanRebaseRepoT(t)
	var wrote []core.CompositionVerdictInput
	fx := newCompositionFixture(dir,
		func(context.Context, string, string) (core.CompositionAuditSnapshot, error) {
			return core.CompositionAuditSnapshot{
				LaneAuditRef: "audit-artifact-sha",
				AuditedBase:  "old-main-sha",
				Diff:         []byte("this is not the real diff"),
				PatchID:      "0000000000000000000000000000000000dead", // deliberately wrong
			}, nil
		},
		func(context.Context, string) map[string]string { return greenComposedGateResults() },
		func(ledgerPath string, in core.CompositionVerdictInput) error {
			wrote = append(wrote, in)
			return nil
		},
	)

	if err := runCompositionCycle(t, fx.o); err != nil {
		t.Fatalf("a rejected composition attempt must still recover via the pre-existing full-audit path, got: %v", err)
	}
	if fx.ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail once → full re-audit → reship, unchanged pre-existing path)", fx.ship.calls)
	}
	// Builder correction (cycle 801): baseline is 1 initial + 1 recovery
	// re-audit = 2 total for one fallback (see the note in the sibling
	// success test above and TestOrchestrator_RecoveryDepthBudget).
	if fx.audit.calls != 2 {
		t.Fatalf("audit calls = %d, want 2 — patch-id drift must fall back to the existing full re-audit (1 initial + 1 recovery)", fx.audit.calls)
	}
	if len(wrote) != 0 {
		t.Fatalf("compositionVerdictWriter must NOT be called on patch-id drift, got %d calls", len(wrote))
	}
}

// TestRecoverFromShipError_CleanRebase_MissingComposedGateFallsBackToFullAudit
// is a companion negative case: even with a matching patch-id, a RED/missing
// required composed gate (ciparity.RequiredComposedGates) must reject the
// fast path exactly like ship's own reader does — gates follow the tree.
func TestRecoverFromShipError_CleanRebase_MissingComposedGateFallsBackToFullAudit(t *testing.T) {
	dir, preDiff := initCleanRebaseRepoT(t)
	patchID, err := ledger.PatchID(preDiff)
	if err != nil {
		t.Fatalf("compute fixture patch-id: %v", err)
	}
	var wrote []core.CompositionVerdictInput
	fx := newCompositionFixture(dir,
		func(context.Context, string, string) (core.CompositionAuditSnapshot, error) {
			return core.CompositionAuditSnapshot{
				LaneAuditRef: "audit-artifact-sha",
				AuditedBase:  "old-main-sha",
				Diff:         preDiff,
				PatchID:      patchID,
			}, nil
		},
		func(context.Context, string) map[string]string {
			red := greenComposedGateResults()
			red["apicover"] = "fail" // one required gate is red
			return red
		},
		func(ledgerPath string, in core.CompositionVerdictInput) error {
			wrote = append(wrote, in)
			return nil
		},
	)

	if err := runCompositionCycle(t, fx.o); err != nil {
		t.Fatalf("a rejected composition attempt must still recover via the pre-existing full-audit path, got: %v", err)
	}
	// Builder correction (cycle 801): baseline is 1 initial + 1 recovery
	// re-audit = 2 total (see note above).
	if fx.audit.calls != 2 {
		t.Fatalf("audit calls = %d, want 2 — a red required composed gate must fall back to full re-audit (1 initial + 1 recovery)", fx.audit.calls)
	}
	if len(wrote) != 0 {
		t.Fatalf("compositionVerdictWriter must NOT be called when a required gate is red, got %d calls", len(wrote))
	}
}

// TestRecoverFromShipError_CleanRebase_WriterFailureFallsBackToFullAudit
// pins fail-closed behavior: a ledger I/O error from the injected writer
// must never abort the cycle — it degrades to the existing full re-audit,
// exactly like every other rejection path.
func TestRecoverFromShipError_CleanRebase_WriterFailureFallsBackToFullAudit(t *testing.T) {
	dir, preDiff := initCleanRebaseRepoT(t)
	patchID, err := ledger.PatchID(preDiff)
	if err != nil {
		t.Fatalf("compute fixture patch-id: %v", err)
	}
	fx := newCompositionFixture(dir,
		func(context.Context, string, string) (core.CompositionAuditSnapshot, error) {
			return core.CompositionAuditSnapshot{
				LaneAuditRef: "audit-artifact-sha",
				AuditedBase:  "old-main-sha",
				Diff:         preDiff,
				PatchID:      patchID,
			}, nil
		},
		func(context.Context, string) map[string]string { return greenComposedGateResults() },
		func(string, core.CompositionVerdictInput) error {
			return errWriteBoom
		},
	)

	if err := runCompositionCycle(t, fx.o); err != nil {
		t.Fatalf("a writer failure must fail closed into the existing full-audit recovery, not abort, got: %v", err)
	}
	// Builder correction (cycle 801): baseline is 1 initial + 1 recovery
	// re-audit = 2 total (see note above).
	if fx.audit.calls != 2 {
		t.Fatalf("audit calls = %d, want 2 — a writer failure must fall back to full re-audit (1 initial + 1 recovery)", fx.audit.calls)
	}
}
