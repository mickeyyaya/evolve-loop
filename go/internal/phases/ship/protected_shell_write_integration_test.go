//go:build integration

package ship

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

func TestAShellWrittenProtectedFileIsHeldByTheBuildFloorAndTheShipTripwire(t *testing.T) {
	t.Parallel()
	const protected = "go/internal/guards/role.go"
	repo := gittest.Fixture(t)
	repo.Write(protected, "package guards\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")

	write := exec.Command("sh", "-c", "printf x >> "+protected)
	write.Dir = repo.Dir
	if out, err := write.CombinedOutput(); err != nil {
		t.Fatalf("shell write: %v\n%s", err, out)
	}

	floor := core.BuildHandoffFloor{{Name: "protected", Run: core.ProtectedSurfaceFloorChecks(guards.IsProtectedSurface)}}
	verdict := floor.Review(context.Background(), core.ReviewInput{
		Phase:           string(core.PhaseBuild),
		Worktree:        repo.Dir,
		WorktreeBaseSHA: base,
		Protected:       guards.IsProtectedSurface,
	})
	if verdict.Approve {
		t.Fatalf("the build handoff floor approved an unreverted shell write to %s", protected)
	}
	if len(verdict.Failures) != 1 || !strings.Contains(verdict.Failures[0], protected) {
		t.Errorf("the floor rejection must name %s once, got %q", protected, verdict.Failures)
	}

	opts := &Options{ProjectRoot: repo.Dir, Class: ClassCycle, Runner: execRunner}
	var res RunResult
	err := verifyClass(context.Background(), opts, &res)
	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeControlPlaneViolation {
		t.Fatalf("the ship tripwire must refuse the same worktree with CodeControlPlaneViolation, got %v", err)
	}
	if se.Debug["protected_paths"] != protected {
		t.Errorf("the tripwire refusal must name %s, got %v", protected, se.Debug)
	}
}
