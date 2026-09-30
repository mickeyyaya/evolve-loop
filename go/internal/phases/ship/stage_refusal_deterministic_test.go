package ship

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

// stagingRefusalRunner scripts `git status --porcelain` to report the given
// changed paths and makes `git add` refuse with a stderr shape the
// stderr-based classifier cannot place, isolating the two-strikes memo under
// test from stderr classification.
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

func stageAndExpectFailure(t *testing.T, workspace, porcelain string) *core.ShipError {
	t.Helper()
	r := stagingRefusalRunner(porcelain)
	if workspace != "" {
		mustWrite(t, filepath.Join(workspace, "build-report.md"), "`"+strings.Join(shipmanifest.ChangedPaths(porcelain), "` `")+"`\n")
	}
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

func TestStageRefusal_FirstStrikeStaysTransient(t *testing.T) {
	se := stageAndExpectFailure(t, t.TempDir(), " M .evolve/evals/foo.md\n")
	if se.Class != core.ShipClassTransient {
		t.Errorf("first refusal class = %q, want %q — one strike must still be retryable", se.Class, core.ShipClassTransient)
	}
}

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

func TestStageRefusal_NoWorkspaceStaysTransient(t *testing.T) {
	const porcelain = " M .evolve/evals/foo.md\n"
	for i := 0; i < 2; i++ {
		if se := stageAndExpectFailure(t, "", porcelain); se.Class != core.ShipClassTransient {
			t.Errorf("attempt %d with no workspace: class = %q, want %q", i+1, se.Class, core.ShipClassTransient)
		}
	}
}
