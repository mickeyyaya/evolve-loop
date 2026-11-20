package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
)

const (
	testAuditFloorPrefix = "test-audit ratchet: "
	testAuditPkg         = "./internal/testaudit"
)

func testAuditFailures(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	paths, err := worktreePathsSince(ctx, in.Worktree, floorBase(in), "--no-renames")
	if err != nil {
		return []string{fmt.Sprintf("%sthe changed paths cannot be read (%v) — the audit cannot tell which packages changed; fix the git state and run the floor again", testAuditFloorPrefix, err)}
	}
	scope := testaudit.AuditScope(paths)
	moduleDir := codequality.ModuleDir(in.Worktree)
	if scope.Whole {
		return wholeTestAuditFailures(ctx, moduleDir)
	}
	if len(scope.Packages) == 0 {
		return nil
	}
	cfg, err := testaudit.LoadConfig(moduleDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("%sthe audit config of the worktree cannot be loaded: %v", testAuditFloorPrefix, err)}
	}
	var out []string
	for _, problem := range testaudit.Problems(testaudit.CheckPackages(moduleDir, scope.Packages, cfg)) {
		out = append(out, testAuditLine(problem))
	}
	return out
}

func wholeTestAuditFailures(ctx context.Context, moduleDir string) []string {
	output, passed := buildSelfCheckRunner(ctx, moduleDir, testAuditPkg)
	if passed {
		return nil
	}
	return []string{fmt.Sprintf("%sthe audit rules, config or central baselines changed, so %s ran in the worktree and is RED:\n%s", testAuditFloorPrefix, testAuditPkg, floorFailureDiagnostic(output))}
}

func testAuditLine(problem error) string {
	var row *testaudit.RowError
	if !errors.As(problem, &row) {
		return testAuditFloorPrefix + problem.Error()
	}
	if errors.Is(problem, testaudit.ErrStaleBaselineRow) {
		return fmt.Sprintf("%s%v — drop the stale row: run `evolve test audit --rebaseline %s` (or delete the row from the central baseline file when the package is gone)", testAuditFloorPrefix, problem, row.Finding.Package)
	}
	return fmt.Sprintf("%s%v — remove the weak pattern (for error text: a sentinel error and errors.Is); Rebaseline refuses new rows", testAuditFloorPrefix, problem)
}
