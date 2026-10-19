package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

var cycleReauditAuditRunner = defaultCycleReauditAuditRunner

func defaultCycleReauditAuditRunner(deps orchDeps) (core.PhaseRunner, error) {
	if deps.RoutingErr != nil {
		return nil, deps.RoutingErr
	}
	runner, ok := deps.Runners[core.PhaseAudit]
	if !ok {
		return nil, errors.New("no audit runner is wired")
	}
	return runner, nil
}

func runCycleReaudit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle reaudit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		evolveDir   string
		cycle       int
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to the project root (default cwd)")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ state directory (default <project-root>/.evolve)")
	fs.IntVar(&cycle, "cycle", 0, "cycle to re-audit on the current base (required)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if cycle <= 0 {
		fmt.Fprintln(stderr, "evolve cycle reaudit: --cycle N (N > 0) is required")
		return 10
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, cycleResetRootWarner(stderr))
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	deps := wireOrchestratorDepsFn(projectRoot, evolveDir, io.Discard, routingRun{env: filterEvolveEnv(os.Environ()), role: roleCycle})
	defer deps.closeSignals()
	audit, err := cycleReauditAuditRunner(deps)
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle reaudit: %v\n", err)
		return 1
	}
	res, err := core.ReauditCycle(context.Background(), core.ReauditInput{
		ProjectRoot: projectRoot,
		EvolveDir:   evolveDir,
		Cycle:       cycle,
		Ledger:      ledger.New(evolveDir),
		Audit:       audit,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evolve cycle reaudit: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "evolve cycle reaudit: cycle %d verdict=%s base=%s verdict_path=%s\n", res.Cycle, res.Verdict, res.NewBase, res.VerdictPath)
	return 0
}
