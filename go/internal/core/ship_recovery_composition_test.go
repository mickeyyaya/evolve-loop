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

var errWriteBoom = errors.New("ledger write boom")

func writeFileT(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

type fixedWorktree struct{ dir string }

func (f fixedWorktree) Create(string, int) (string, error) { return f.dir, nil }
func (f fixedWorktree) Cleanup(string, string) error       { return nil }

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

func greenComposedGateResults() map[string]ciparity.GateOutcome {
	out := map[string]ciparity.GateOutcome{}
	for _, g := range ciparity.RequiredComposedGates {
		out[g] = ciparity.GateOutcome{Status: "pass"}
	}
	return out
}

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
	write("peer.txt", "peer change\n")
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-m", "peer change")
	runGitT(t, dir, "checkout", "cycle")
	return dir, preRebaseDiff
}

type compositionCycleFixture struct {
	ship    *shipErrorStub
	audit   *countingRunner
	build   *explanationWritingRunner
	storage *recStorage
	o       *core.Orchestrator
}

func newCompositionFixture(
	repoDir string,
	snap func(context.Context, string, string) (core.CompositionAuditSnapshot, error),
	gates func(context.Context, string) map[string]ciparity.GateOutcome,
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

func TestRecoverFromShipError_CleanRebase_RebuildsBaseBoundExplanationBeforeReaudit(t *testing.T) {
	t.Parallel()

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
		func(context.Context, string) map[string]ciparity.GateOutcome { return greenComposedGateResults() },
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

func TestRecoverFromShipError_CleanRebase_PatchIdDriftFallsBackToFullAudit(t *testing.T) {
	t.Parallel()

	dir, _ := initCleanRebaseRepoT(t)
	var wrote []core.CompositionVerdictInput
	fx := newCompositionFixture(dir,
		func(context.Context, string, string) (core.CompositionAuditSnapshot, error) {
			return core.CompositionAuditSnapshot{
				LaneAuditRef: "audit-artifact-sha",
				AuditedBase:  "old-main-sha",
				Diff:         []byte("this is not the real diff"),
				PatchID:      "0000000000000000000000000000000000dead",
			}, nil
		},
		func(context.Context, string) map[string]ciparity.GateOutcome { return greenComposedGateResults() },
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
	if fx.audit.calls != 2 {
		t.Fatalf("audit calls = %d, want 2 — patch-id drift must fall back to the existing full re-audit (1 initial + 1 recovery)", fx.audit.calls)
	}
	if len(wrote) != 0 {
		t.Fatalf("compositionVerdictWriter must NOT be called on patch-id drift, got %d calls", len(wrote))
	}
}

func TestRecoverFromShipError_CleanRebase_MissingComposedGateFallsBackToFullAudit(t *testing.T) {
	t.Parallel()

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
		func(context.Context, string) map[string]ciparity.GateOutcome {
			red := greenComposedGateResults()
			red["apicover"] = ciparity.GateOutcome{Status: "fail"}
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
	if fx.audit.calls != 2 {
		t.Fatalf("audit calls = %d, want 2 — a red required composed gate must fall back to full re-audit (1 initial + 1 recovery)", fx.audit.calls)
	}
	if len(wrote) != 0 {
		t.Fatalf("compositionVerdictWriter must NOT be called when a required gate is red, got %d calls", len(wrote))
	}
}

func TestRecoverFromShipError_CleanRebase_WriterFailureFallsBackToFullAudit(t *testing.T) {
	t.Parallel()

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
		func(context.Context, string) map[string]ciparity.GateOutcome { return greenComposedGateResults() },
		func(string, core.CompositionVerdictInput) error {
			return errWriteBoom
		},
	)

	if err := runCompositionCycle(t, fx.o); err != nil {
		t.Fatalf("a writer failure must fail closed into the existing full-audit recovery, not abort, got: %v", err)
	}
	if fx.audit.calls != 2 {
		t.Fatalf("audit calls = %d, want 2 — a writer failure must fall back to full re-audit (1 initial + 1 recovery)", fx.audit.calls)
	}
}
