package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	cycleRetireKind          = "retire"
	cycleRetireReleaseReason = "cycle-retired"
)

type cycleRetireResult struct {
	Cycle    int    `json:"cycle"`
	Phase    string `json:"phase"`
	Reason   string `json:"reason"`
	State    string `json:"state"`
	Released int    `json:"released_claims"`
}

func runCycleRetire(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve cycle retire", flag.ContinueOnError)
	fs.SetOutput(stderr)
	projectRoot := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	cycle := fs.Int("cycle", 0, "the paused cycle to retire")
	reason := fs.String("reason", "operator-requested retire", "reason recorded in the project ledger")
	asJSON := fs.Bool("json", false, "print the result as JSON")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || *cycle <= 0 {
		fmt.Fprintln(stderr, "usage: evolve cycle retire --cycle N [--reason R] [--json] [--project-root P]")
		return cliExitUsage
	}
	root, err := loopStopRoot(*projectRoot, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "cycle retire: cwd: %v\n", err)
		return cliExitIO
	}
	evolveDir := paths.EvolveDirOf(root)
	runDir := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(*cycle))
	if lease, live := runlease.LiveOwner(runDir, time.Now()); live {
		fmt.Fprintf(stderr, "cycle retire: refused: cycle %d has a live lane (run %s, pid %d); stop the wave first: evolve wave stop --wait 10m\n", *cycle, lease.RunID, lease.OwnerPID)
		return cliExitRefused
	}
	paused, err := core.PausedLaneScopes(root, evolveDir, core.ResumeOptions{Log: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "cycle retire: cannot list the paused cycles: %v\n", err)
		return cliExitIO
	}
	target, found := findPausedCycle(paused, *cycle)
	if !found {
		fmt.Fprintf(stderr, "cycle retire: cycle %d is not a paused cycle; nothing retired\n", *cycle)
		return cliExitInput
	}
	signals, closeSignals := newRoleSignalCenter("retire", root, evolveDir, stderr)
	defer closeSignals()
	result, rc := retirePausedCycle(root, evolveDir, target, *reason, signals, stderr)
	if rc != cliExitOK {
		return rc
	}
	if *asJSON {
		return printVerbJSON("cycle retire", result, stdout, stderr)
	}
	fmt.Fprintf(stdout, "retired cycle %d (phase=%s): %d inbox claim(s) released; worktree kept for gc; reason: %s\n", result.Cycle, result.Phase, result.Released, result.Reason)
	return cliExitOK
}

func findPausedCycle(paused []core.PausedScope, cycle int) (core.PausedScope, bool) {
	for _, p := range paused {
		if p.Cycle == cycle {
			return p, true
		}
	}
	return core.PausedScope{}, false
}

func retirePausedCycle(root, evolveDir string, target core.PausedScope, reason string, signals *signalcenter.Center, stderr io.Writer) (cycleRetireResult, int) {
	appender := ledger.New(evolveDir, ledger.WithSignals(signals))
	entry := core.LedgerEntry{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Cycle:      target.Cycle,
		CycleLabel: fmt.Sprintf("retire-cycle-%d", target.Cycle),
		Role:       "operator",
		Kind:       cycleRetireKind,
		Message:    reason,
	}
	if err := appender.Append(context.Background(), entry); err != nil {
		fmt.Fprintf(stderr, "cycle retire: cycle %d: ledger: %v\n", target.Cycle, err)
		return cycleRetireResult{}, cliExitFailed
	}
	if _, err := checkpoint.RetireCheckpoint(target.StatePath); err != nil {
		fmt.Fprintf(stderr, "cycle retire: cycle %d: revoke the resume grant: %v\n", target.Cycle, err)
		return cycleRetireResult{}, cliExitFailed
	}
	released, err := inboxmover.ReleaseCycleProcessingWithReason(inboxmover.Options{ProjectRoot: root, Ledger: appender, Stderr: stderr, Signals: signals}, target.Cycle, cycleRetireReleaseReason)
	if err != nil {
		fmt.Fprintf(stderr, "cycle retire: WARN: cycle %d retired, but its inbox claims were not released: %v\n", target.Cycle, err)
	}
	return cycleRetireResult{Cycle: target.Cycle, Phase: target.Phase, Reason: reason, State: target.StatePath, Released: released.Recovered}, cliExitOK
}
