package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// runBranches audits and prunes superseded local cycle-* branches.
func runBranches(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve branches: missing subcommand (audit|prune)")
		return 10
	}
	switch args[0] {
	case "audit":
		return runBranchesAudit(args[1:], stdout, stderr)
	case "prune":
		return runBranchesPrune(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve branches: unknown subcommand %q\n", args[0])
		return 10
	}
}

// branchesFlags parses both subcommands' flags; audit ignores --dry-run.
func branchesFlags(name string, args []string, stderr io.Writer) (projectRoot, base string, dryRun bool, err error) {
	fs := flag.NewFlagSet("evolve branches "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&base, "base", "main", "base branch a cycle-* ref is measured against")
	fs.BoolVar(&dryRun, "dry-run", true, "report only; never delete (prune only)")
	if err = fs.Parse(args); err != nil {
		return "", "", true, err
	}
	projectRoot = paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve branches: WARN: %s\n", m)
	})
	return projectRoot, base, dryRun, nil
}

// alwaysOpenPR makes a walk read-only: PruneSupersededOrphans never deletes a
// ref that has an open PR.
func alwaysOpenPR(string) (bool, error) { return true, nil }

func runBranchesAudit(args []string, stdout, stderr io.Writer) int {
	projectRoot, base, _, err := branchesFlags("audit", args, stderr)
	if err != nil {
		return 10
	}
	ctx := context.Background()
	verdicts, err := core.PruneSupersededOrphans(ctx, projectRoot, base, alwaysOpenPR)
	if err != nil {
		fmt.Fprintf(stderr, "evolve branches audit: %v\n", err)
		return 1
	}
	for _, v := range verdicts {
		landable, err := core.CarryforwardCandidateLandable(ctx, projectRoot, v.Ref, base)
		if err != nil {
			fmt.Fprintf(stderr, "evolve branches audit: landable(%s): %v\n", v.Ref, err)
			return 1
		}
		fmt.Fprintf(stdout, "%s superseded=%t landable=%t\n", v.Ref, v.Superseded, landable)
	}
	return 0
}

func runBranchesPrune(args []string, stdout, stderr io.Writer) int {
	projectRoot, base, dryRun, err := branchesFlags("prune", args, stderr)
	if err != nil {
		return 10
	}
	ctx := context.Background()

	hasOpenPR := alwaysOpenPR
	if !dryRun {
		hasOpenPR = remoteOpenPR(projectRoot)
	}
	verdicts, err := core.PruneSupersededOrphans(ctx, projectRoot, base, hasOpenPR)
	if err != nil {
		fmt.Fprintf(stderr, "evolve branches prune: %v\n", err)
		return 1
	}
	for _, v := range verdicts {
		switch {
		case !v.Superseded:
			fmt.Fprintf(stdout, "%s superseded=false kept\n", v.Ref)
		case v.Pruned:
			fmt.Fprintf(stdout, "%s superseded=true pruned\n", v.Ref)
		case dryRun:
			fmt.Fprintf(stdout, "%s superseded=true would-prune\n", v.Ref)
		default:
			fmt.Fprintf(stdout, "%s superseded=true kept-open-pr\n", v.Ref)
		}
	}
	return 0
}

// remoteOpenPR reports no open PR when there is no remote or no gh CLI, since
// no remote PR can then protect the ref.
func remoteOpenPR(dir string) func(ref string) (bool, error) {
	return func(ref string) (bool, error) {
		if !hasGitRemote(dir) {
			return false, nil
		}
		if _, err := exec.LookPath("gh"); err != nil {
			return false, nil
		}
		out, err := exec.Command("gh", "-C", dir, "pr", "list", "--head", ref, "--state", "open", "--json", "number", "-q", "length").Output()
		if err != nil {
			return false, fmt.Errorf("gh pr list --head %s: %w", ref, err)
		}
		trimmed := strings.TrimSpace(string(out))
		return trimmed != "0" && trimmed != "", nil
	}
}

func hasGitRemote(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "remote").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
