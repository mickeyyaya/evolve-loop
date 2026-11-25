package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeBuildDerivedFloorSkipped signalcenter.Code = "BUILD_DERIVED_FLOOR_SKIPPED"

const (
	DerivedCheckDeadline    = 120 * time.Second
	derivedFindingOutputMax = 400
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildDerivedFloorSkipped, "the build floor skipped the derived-file checks because a check verb could not run (no toolchain, or the check passed its deadline). The floor passes this check and the ship gate stays the backstop. fields.cause")
}

type DerivedRunFn func(worktree string) derived.Generator

func StaleDerivedFindings(ctx context.Context, worktree string, changed []string, run derived.Generator, warn io.Writer) ([]string, error) {
	entries := staleDerivedEntries(ctx, worktree, changed, warn)
	ctx, cancel := context.WithTimeout(ctx, DerivedCheckDeadline)
	defer cancel()
	var lines []string
	for _, e := range entries {
		runErr := run(ctx, e.Check...)
		var exitErr *exec.ExitError
		switch {
		case runErr == nil:
		case ctx.Err() != nil || !errors.As(runErr, &exitErr):
			return nil, fmt.Errorf("the check of derived entry %s did not run: %w", e.Name, runErr)
		default:
			lines = append(lines, derivedFinding(e, runErr))
		}
	}
	return lines, nil
}

func derivedFinding(e derived.Entry, checkErr error) string {
	output := checkErr.Error()
	if len(output) > derivedFindingOutputMax {
		output = output[len(output)-derivedFindingOutputMax:]
	}
	return fmt.Sprintf("derived file RED: %s failed its check (%s). Fix the input the check names, then run `evolve %s` from the worktree and stage its outputs (%s), then build again.",
		e.Name, output, strings.Join(e.Generate, " "), strings.Join(e.Pathspecs(), " "))
}

func DerivedFloorChecks(run DerivedRunFn) BuildFloorCheckFn {
	return FailOpen("derived", derivedFloorJudge(run))
}

func derivedFloorJudge(run DerivedRunFn) FloorJudge {
	return func(ctx context.Context, in ReviewInput) ([]string, error) {
		paths, err := floorPaths(ctx, in)
		if err != nil || len(paths) == 0 {
			return nil, err
		}
		lines, err := StaleDerivedFindings(ctx, in.Worktree, paths, run(in.Worktree), in.errOut())
		if err != nil {
			in.reportDerivedSkipped(err)
		}
		return lines, nil
	}
}

func (in ReviewInput) reportDerivedSkipped(cause error) {
	reason := fmt.Sprintf("build floor: the derived-file checks did not run (%v); the ship gate still enforces them", cause)
	fmt.Fprintf(in.errOut(), "[build-floor] WARN %s\n", reason)
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.reportDerivedSkipped", Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityWarn, Code: CodeBuildDerivedFloorSkipped, Reason: reason,
		Fields: map[string]string{"cause": cause.Error()},
	})
}
