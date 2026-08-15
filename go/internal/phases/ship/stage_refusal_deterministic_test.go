package ship

// stage_refusal_deterministic_test.go — RED contract for cycle-1440 task
// `deterministic-stage-refusal-router`.
//
// Defect (cycle-1365, live): stageExplicitPaths classifies EVERY `git add`
// refusal as core.ShipClassTransient, so the failure floor keeps re-dispatching
// a refusal that can never succeed in place. Cycle 1365 burned its whole retry
// budget on the SAME .evolve/evals pathspec refused twice — its worktree base
// predated the .gitignore carve-out, so no retry could ever win.
//
// Contract under test (not yet implemented — RED until Builder adds the
// two-strikes rule): the FIRST refusal of a given pathspec stays TRANSIENT (a
// genuinely flaky add must keep its retry), and a SECOND CONSECUTIVE refusal of
// the SAME pathspec is reclassified core.ShipClassPrecondition — deterministic,
// so the router stops burning attempts and routes to continuation/salvage. A
// refusal of a DIFFERENT pathspec is a different failure and resets to transient.
//
// The refusal memory is per-workspace (opts.WorkspacePath), which is what makes
// "consecutive" observable across the separate ship attempts of one cycle.

import (
	"context"
	"io"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// stagingRefusalRunner scripts `git status --porcelain` to report the given
// changed paths and makes `git add` refuse with an UNRECOGNISED stderr shape.
//
// Cycle-1473 re-base: this file's fixture was git's rc=1 gitignore-advice
// refusal, which the `gitstage-deterministic-classification` contract now
// classifies non-transient on the FIRST failure from captured git_stderr (see
// stage_classify_stderr_test.go). Keeping that fixture here would assert two
// contradictory classes for one stderr. The two-strikes rule these tests exist
// to pin is orthogonal to the stderr shape, so they now run on a stderr the
// classifier cannot place — which is exactly where the strike memo is still the
// only signal, and their original intent (a first, possibly-flaky refusal keeps
// its retry; the same pathspec twice does not) is preserved unchanged.
func stagingRefusalRunner(porcelain string) *scriptedRunner {
	r := &scriptedRunner{scripts: map[string]struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{}}
	r.scripts["git status"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stdout: porcelain}
	r.scripts["git add"] = struct {
		stdout string
		stderr string
		exit   int
		err    error
	}{stderr: "error: an add failure shape the stderr classifier cannot place\n", exit: 128}
	return r
}

// stageAndExpectFailure runs stageExplicitPaths against a refusing runner and
// returns the ShipError it must produce.
func stageAndExpectFailure(t *testing.T, workspace, porcelain string) *core.ShipError {
	t.Helper()
	r := stagingRefusalRunner(porcelain)
	opts := &Options{ProjectRoot: t.TempDir(), WorkspacePath: workspace, Runner: r.runner(), Stderr: io.Discard}
	err := stageExplicitPaths(context.Background(), opts, &RunResult{}, "")
	if err == nil {
		t.Fatalf("a refused `git add` must produce a ship error, got nil")
	}
	se, ok := core.AsShipError(err)
	if !ok {
		t.Fatalf("staging failure must be a typed ShipError, got %T: %v", err, err)
	}
	if se.Code != core.CodeGitStageFailed {
		t.Fatalf("code = %q, want %q", se.Code, core.CodeGitStageFailed)
	}
	return se
}

// TestStageRefusal_FirstStrikeStaysTransient is the negative-side pin: the
// two-strikes rule must not turn a first, possibly-flaky refusal into a
// deterministic block — that would delete the retry the ladder depends on.
func TestStageRefusal_FirstStrikeStaysTransient(t *testing.T) {
	se := stageAndExpectFailure(t, t.TempDir(), " M .evolve/evals/foo.md\n")
	if se.Class != core.ShipClassTransient {
		t.Errorf("first refusal class = %q, want %q — one strike must still be retryable", se.Class, core.ShipClassTransient)
	}
}

// TestStageRefusal_SecondSamePathspecIsDeterministic is the primary case: the
// same pathspec refused twice in the SAME workspace is unwinnable in place.
func TestStageRefusal_SecondSamePathspecIsDeterministic(t *testing.T) {
	ws := t.TempDir()
	const porcelain = " M .evolve/evals/foo.md\n"

	if first := stageAndExpectFailure(t, ws, porcelain); first.Class != core.ShipClassTransient {
		t.Fatalf("precondition: first refusal class = %q, want %q", first.Class, core.ShipClassTransient)
	}
	second := stageAndExpectFailure(t, ws, porcelain)
	if second.Class != core.ShipClassPrecondition {
		t.Errorf("second consecutive refusal of the SAME pathspec class = %q, want %q — cycle-1365 burned the full retry budget on exactly this shape",
			second.Class, core.ShipClassPrecondition)
	}
}

// TestStageRefusal_DifferentPathspecStaysTransient is the load-bearing negative:
// a rule that simply counts refusals (rather than matching the pathspec) would
// pass the test above while wrongly killing the retry for an unrelated second
// failure. Same workspace, different refused pathspec → still transient.
func TestStageRefusal_DifferentPathspecStaysTransient(t *testing.T) {
	ws := t.TempDir()

	if first := stageAndExpectFailure(t, ws, " M .evolve/evals/foo.md\n"); first.Class != core.ShipClassTransient {
		t.Fatalf("precondition: first refusal class = %q, want %q", first.Class, core.ShipClassTransient)
	}
	second := stageAndExpectFailure(t, ws, " M docs/architecture/control-flags.md\n")
	if second.Class != core.ShipClassTransient {
		t.Errorf("a DIFFERENT refused pathspec class = %q, want %q — two-strikes must match the pathspec, not merely count failures",
			second.Class, core.ShipClassTransient)
	}
}

// TestStageRefusal_SeparateWorkspacesDoNotShareStrikes is the isolation edge
// case: fleet lanes run concurrently, so one lane's first strike must never
// deterministically block a peer lane's first strike.
func TestStageRefusal_SeparateWorkspacesDoNotShareStrikes(t *testing.T) {
	const porcelain = " M .evolve/evals/foo.md\n"

	if first := stageAndExpectFailure(t, t.TempDir(), porcelain); first.Class != core.ShipClassTransient {
		t.Fatalf("precondition: lane A first refusal class = %q, want %q", first.Class, core.ShipClassTransient)
	}
	other := stageAndExpectFailure(t, t.TempDir(), porcelain)
	if other.Class != core.ShipClassTransient {
		t.Errorf("peer lane's FIRST refusal class = %q, want %q — strike memory must be workspace-scoped",
			other.Class, core.ShipClassTransient)
	}
}

// TestStageRefusal_NoWorkspaceStaysTransient pins the degrade path: with no
// workspace there is nowhere to record a strike, so the classification must fall
// back to today's transient behavior rather than guessing deterministic.
func TestStageRefusal_NoWorkspaceStaysTransient(t *testing.T) {
	const porcelain = " M .evolve/evals/foo.md\n"
	for i := 0; i < 2; i++ {
		if se := stageAndExpectFailure(t, "", porcelain); se.Class != core.ShipClassTransient {
			t.Errorf("attempt %d with no workspace: class = %q, want %q", i+1, se.Class, core.ShipClassTransient)
		}
	}
}
