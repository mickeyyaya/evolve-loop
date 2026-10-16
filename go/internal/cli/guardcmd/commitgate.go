package guardcmd

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

func RunCommitGate(args []string, _ io.Reader, stdout, stderr io.Writer) int {
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
