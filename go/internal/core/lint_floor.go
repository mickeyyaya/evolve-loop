package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeBuildLintFloorSkipped signalcenter.Code = "BUILD_LINT_FLOOR_SKIPPED"

const lintFloorFindingsMax = 1500

var (
	ErrLintToolMissing = errors.New("golangci-lint is not installed")
	ErrLintLockWait    = errors.New("golangci-lint was stopped while it waited for another run's lock or its own lint")
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBuild, CodeBuildLintFloorSkipped, "the build floor did not run golangci-lint on the changed packages because the tool is missing or failed to run, so the floor passes this check and the CI lint job stays the backstop; fields.cause")
}

type LintFn func(ctx context.Context, moduleDir string, pkgs []string) (findings string, err error)

func LintFloorChecks(run LintFn) BuildFloorCheckFn {
	return FailOpen("lint", lintFloorJudge(run))
}

func lintFloorJudge(run LintFn) FloorJudge {
	return func(ctx context.Context, in ReviewInput) ([]string, error) {
		paths, err := floorPaths(ctx, in)
		if err != nil {
			return nil, err
		}
		moduleDir := codequality.ModuleDir(in.Worktree)
		pkgs := buildTagVisiblePackages(ctx, moduleDir, changedGoTestPackages(paths), in.errOut())
		if len(pkgs) == 0 {
			return nil, nil
		}
		findings, err := run(ctx, moduleDir, pkgs)
		pkgList := strings.Join(pkgs, " ")
		switch {
		case errors.Is(err, ErrLintLockWait):
			return []string{fmt.Sprintf("golangci-lint RED (class: retryable): the lint did not judge your code. %v. Build again; the lint runs on the changed packages %s", err, pkgList)}, nil
		case err != nil:
			in.reportLintSkipped(err)
			return nil, nil
		case findings == "":
			return nil, nil
		}
		return []string{fmt.Sprintf("golangci-lint RED on %s — the CI lint job refuses these findings. Fix each finding (file:line below), then run `golangci-lint run %s` from go/:\n%s", pkgList, pkgList, capKeepingCorrection(findings, lintFloorFindingsMax))}, nil
	}
}

func (in ReviewInput) reportLintSkipped(cause error) {
	reason := fmt.Sprintf("build floor: golangci-lint did not run on the changed packages (%v); the CI lint job still enforces it", cause)
	fmt.Fprintf(in.errOut(), "[build-floor] WARN %s\n", reason)
	in.Signals.Emit(signalcenter.Event{
		Cycle: in.Cycle, RunID: in.RunID, Phase: string(PhaseBuild),
		Module: signalcenter.ModuleBuild, Origin: "ReviewInput.reportLintSkipped", Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityWarn, Code: CodeBuildLintFloorSkipped, Reason: reason,
		Fields: map[string]string{"cause": cause.Error()},
	})
}
