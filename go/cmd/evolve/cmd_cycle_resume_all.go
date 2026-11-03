package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type cycleResumeGrant struct {
	Cycle     int      `json:"cycle"`
	Phase     string   `json:"phase"`
	Scope     []string `json:"scope"`
	Granted   bool     `json:"granted"`
	HasBudget bool     `json:"has_budget"`
}

type cycleResumeAllResult struct {
	DryRun bool               `json:"dry_run"`
	Cycles []cycleResumeGrant `json:"cycles"`
}

var cycleGrantResume = checkpoint.GrantOperatorResume

func runCycleResumeAll(root string, dryRun, asJSON bool, stdout, stderr io.Writer) int {
	paused, err := core.PausedLaneScopes(root, paths.EvolveDirOf(root), core.ResumeOptions{Log: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "cycle resume --all: cannot list the paused cycles: %v\n", err)
		return cliExitIO
	}
	result := cycleResumeAllResult{DryRun: dryRun, Cycles: []cycleResumeGrant{}}
	rc := cliExitOK
	for _, p := range paused {
		grant := cycleResumeGrant{Cycle: p.Cycle, Phase: p.Phase, Scope: p.Scope, HasBudget: checkpoint.BudgetLeft(p.AutoAttempts, p.AutoMaxAttempts)}
		if !dryRun {
			_, granted, err := cycleGrantResume(p.StatePath)
			if err != nil {
				fmt.Fprintf(stderr, "cycle resume --all: cycle %d: grant: %v\n", p.Cycle, err)
				rc = cliExitFailed
			}
			grant.Granted, grant.HasBudget = granted, err == nil && !granted
		}
		result.Cycles = append(result.Cycles, grant)
	}
	if asJSON {
		if jrc := printVerbJSON("cycle resume", result, stdout, stderr); jrc != cliExitOK {
			return jrc
		}
		return rc
	}
	printCycleResumeAll(result, stdout)
	return rc
}

func printCycleResumeAll(result cycleResumeAllResult, stdout io.Writer) {
	if len(result.Cycles) == 0 {
		fmt.Fprintln(stdout, "cycle resume --all: no paused cycle to resume")
		return
	}
	for _, g := range result.Cycles {
		fmt.Fprintf(stdout, "cycle %d phase %s scope %s %s\n", g.Cycle, g.Phase, strings.Join(g.Scope, ","), grantWord(g, result.DryRun))
	}
	fmt.Fprintln(stdout, "next: evolve wave next (each lane resumes its paused cycle at the paused phase)")
}

func grantWord(g cycleResumeGrant, dryRun bool) string {
	switch {
	case g.Granted:
		return "granted"
	case g.HasBudget:
		return "has budget"
	case dryRun:
		return "would grant"
	default:
		return "not granted"
	}
}
