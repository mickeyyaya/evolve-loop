package main

// cmd_commitgate.go — `evolve commit-gate run` — the native port of
// commit-gate/commit-gate-runner.sh (bash->Go migration Wave B1).
//
// Detects the languages of the changed files, validates the --reviewers
// precondition (simplify + one review capability), runs lint + targeted tests,
// and on a full pass writes .commit-gate/attestation.json bound to
// sha256(`git diff HEAD`) — the attestation `evolve ship --class manual`
// verifies. Exit codes mirror the bash runner verbatim: 0 pass, 1 fail/precond,
// 2 git/SHA fatal, 3 tool missing, 10 bad args.
//
// ADDITIVE: the bash runner stays in place until B2 confirms differential parity
// and deletes it. Registry wiring (not edited here per task scope):
//
//	{Name: "commit-gate", Summary: "Pre-commit quality gate (lint + targeted tests + attestation)", Run: runCommitGate},

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func runCommitGate(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "evolve commit-gate: usage: commit-gate run --reviewers \"<csv>\" [--files \"p1 p2\"] [--no-install] [--project-root P]")
		return commitgate.ExitBadArgs
	}
	sub, rest := args[0], args[1:]
	if sub != "run" {
		fmt.Fprintf(stderr, "evolve commit-gate: unknown subcommand %q (want: run)\n", sub)
		return commitgate.ExitBadArgs
	}

	fs := flag.NewFlagSet("commit-gate run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		reviewers   = fs.String("reviewers", "", "CSV of reviewers that ran (simplify + one review capability)")
		files       = fs.String("files", "", "override change detection: whitespace-separated path list")
		noInstall   = fs.Bool("no-install", false, "do not auto-install missing tools (hard-fail instead)")
		projectRoot = fs.String("project-root", "", "git working tree root (default: git rev-parse --show-toplevel)")
	)
	if err := fs.Parse(rest); err != nil {
		return commitgate.ExitBadArgs
	}

	runner := sysexec.DefaultRunner
	root := *projectRoot
	if root == "" {
		out, err := sysexec.Output(context.Background(), runner, "", "git", "rev-parse", "--show-toplevel")
		if err != nil {
			fmt.Fprintln(stderr, "[commit-gate] not in a git repo")
			return commitgate.ExitGitFatal
		}
		root = out
	}

	opts := commitgate.Options{
		RepoRoot:     root,
		Reviewers:    *reviewers,
		Files:        *files,
		NoInstall:    *noInstall,
		AttestDir:    os.Getenv("CG_ATTEST_DIR"),
		Env:          os.Environ(),
		Runner:       runner,
		Now:          time.Now,
		TestInstall:  os.Getenv("CG_TEST_INSTALL"),
		ForceMissing: os.Getenv("CG_TEST_FORCE_MISSING"),
	}

	res := opts.Run(context.Background())
	for _, line := range res.Logs {
		fmt.Fprintln(stderr, line)
	}
	if res.ExitCode == commitgate.ExitPass && res.Attestation != nil {
		fmt.Fprintf(stdout, "%s\n", res.Attestation.TreeStateSHA)
	}
	return res.ExitCode
}
